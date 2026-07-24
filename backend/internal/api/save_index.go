package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"palpanel/internal/db"
	"palpanel/internal/paldefender"
	"palpanel/internal/pallocalize"
	"palpanel/internal/saveindex"
)

func (s Server) saveIndexStatus(c *gin.Context) {
	s.saveIndex.EnsureFresh(c.Request.Context())
	ok(c, s.saveIndex.Status(c.Request.Context()))
}

func (s Server) saveIndexRebuild(c *gin.Context) {
	index, status, err := s.saveIndex.Rebuild(c.Request.Context())
	s.invalidateSaveCaches()
	if errors.Is(err, saveindex.ErrDisabled) {
		ok(c, gin.H{"status": status, "index": index})
		return
	}
	if err != nil {
		ok(c, gin.H{"status": status, "index": index})
		return
	}
	if source, sourceErr := s.store.ActiveSaveSource(c.Request.Context()); sourceErr == nil {
		_ = s.store.UpdateSaveSourceIndex(c.Request.Context(), source.ID, index.Snapshot.Fingerprint, index.Parser, index.Warnings, index.GeneratedAt)
	}
	s.triggerAstrBotCatalogSync()
	ok(c, gin.H{"status": status, "index": index})
}

func (s Server) listSavePlayers(c *gin.Context) {
	index, status, view, overlay, valid, err := s.currentPlayerIndex(c)
	if !valid {
		return
	}
	var online onlinePlayersResult
	if overlay {
		online = s.onlinePlayers(c)
		status = statusWithOnlineState(status, online)
	}
	players := playersForView(index.Players, online, overlay)
	if err != nil && !status.Stale {
		players = []saveindex.Player{}
	}
	players = filterPlayers(players, c)
	limit, offset := limitOffset(c)
	paged, summary := paginate(players, limit, offset)
	ok(c, gin.H{"players": flattenPlayers(paged, online), "status": status, "summary": summary, "view": view})
}

func (s Server) getSavePlayer(c *gin.Context) {
	index, status, view, overlay, valid, err := s.currentPlayerIndex(c)
	if !valid {
		return
	}
	var online onlinePlayersResult
	if overlay {
		online = s.onlinePlayers(c)
		status = statusWithOnlineState(status, online)
	}
	if err != nil && !status.Stale && status.State != "disabled" {
		fail(c, http.StatusServiceUnavailable, "save_index_unavailable", err.Error())
		return
	}
	id := c.Param("id")
	players := playersForView(index.Players, online, overlay)
	for _, player := range players {
		if matchesID(id, player.PlayerUID, player.SteamID) {
			ok(c, gin.H{"player": flattenPlayer(player, online), "status": status, "view": view})
			return
		}
	}
	fail(c, http.StatusNotFound, "player_not_found", "player not found")
}

func (s Server) getSavePlayerInventory(c *gin.Context) {
	index, status, view, _, valid, err := s.currentPlayerIndex(c)
	if !valid {
		return
	}
	if err != nil && !status.Stale && status.State != "disabled" {
		fail(c, http.StatusServiceUnavailable, "save_index_unavailable", err.Error())
		return
	}
	id := c.Param("id")
	items := make([]saveindex.Container, 0)
	for _, container := range index.Containers {
		if container.OwnerType == "player" && matchesID(id, container.OwnerID) {
			items = append(items, container)
		}
	}
	ok(c, gin.H{"containers": flattenContainers(items), "status": status, "view": view})
}

type playerDataView struct {
	Scope         string `json:"scope"`
	SourceID      string `json:"source_id"`
	SourceKind    string `json:"source_kind"`
	SourceName    string `json:"source_name"`
	OnlineOverlay bool   `json:"online_overlay"`
}

func (s Server) currentPlayerIndex(c *gin.Context) (saveindex.Index, saveindex.Status, playerDataView, bool, bool, error) {
	sourceQuery := strings.TrimSpace(c.Query("source"))
	if sourceQuery != "" && sourceQuery != "server" {
		fail(c, http.StatusBadRequest, "player_source_invalid", "source must be server when specified")
		return saveindex.EmptyIndex(), saveindex.Status{}, playerDataView{}, false, false, nil
	}

	source := db.SaveSource{ID: "server", Name: "当前服务器存档", Kind: "server"}
	manager := s.saveIndex
	scope := "active"
	if sourceQuery == "server" {
		scope = "server"
		manager = s.serverSaveIndex
		if stored, err := s.store.GetSaveSource(c.Request.Context(), "server"); err == nil {
			source = stored
		}
	} else {
		active, err := s.store.ActiveSaveSource(c.Request.Context())
		if err != nil {
			fail(c, http.StatusInternalServerError, "save_source_read_failed", err.Error())
			return saveindex.EmptyIndex(), saveindex.Status{}, playerDataView{}, false, false, nil
		}
		source = active
	}

	overlay := source.Kind == "server"
	index, status, err := s.playerIndexCurrent(c, manager, overlay)
	view := playerDataView{Scope: scope, SourceID: source.ID, SourceKind: source.Kind, SourceName: source.Name, OnlineOverlay: overlay}
	return index, status, view, overlay, true, err
}

func (s Server) playerIndexCurrent(c *gin.Context, manager *saveindex.Manager, rebuildIfMissing bool) (saveindex.Index, saveindex.Status, error) {
	manager.EnsureFresh(c.Request.Context())
	index, status, err := manager.Current(c.Request.Context())
	if rebuildIfMissing && err == nil && status.State == "not_indexed" {
		return manager.Rebuild(c.Request.Context())
	}
	return index, status, err
}

func playersForView(players []saveindex.Player, online onlinePlayersResult, overlay bool) []saveindex.Player {
	if overlay {
		return mergeSaveAndOnline(players, online.Players)
	}
	offline := append([]saveindex.Player(nil), players...)
	for position := range offline {
		offline[position].IsOnline = false
		offline[position].Ping = nil
		offline[position].IP = ""
	}
	return offline
}

func (s Server) listSaveGuilds(c *gin.Context) {
	index, status, err := s.currentSaveIndex(c)
	if err != nil && !status.Stale {
		index = saveindex.EmptyIndex()
	}
	online := s.onlinePlayers(c)
	status = statusWithOnlineState(status, online)
	guilds := append([]saveindex.Guild(nil), index.Guilds...)
	applyGuildOnlineCounts(guilds, mergeSaveAndOnline(index.Players, online.Players))
	guilds = filterGuilds(guilds, c)
	guilds = localizeGuilds(guilds)
	limit, offset := limitOffset(c)
	paged, summary := paginate(guilds, limit, offset)
	ok(c, gin.H{"guilds": paged, "status": status, "summary": summary})
}

func (s Server) getSaveGuild(c *gin.Context) {
	index, status, err := s.currentSaveIndex(c)
	if err != nil && !status.Stale && status.State != "disabled" {
		fail(c, http.StatusServiceUnavailable, "save_index_unavailable", err.Error())
		return
	}
	guilds := append([]saveindex.Guild(nil), index.Guilds...)
	online := s.onlinePlayers(c)
	applyGuildOnlineCounts(guilds, mergeSaveAndOnline(index.Players, online.Players))
	id := c.Param("id")
	for _, guild := range guilds {
		if matchesID(id, guild.ID, guild.OwnerPlayerUID, guild.Name, pallocalize.GuildName(guild.Name)) {
			ok(c, gin.H{"guild": localizeGuild(guild), "status": status})
			return
		}
	}
	fail(c, http.StatusNotFound, "guild_not_found", "guild not found")
}

func (s Server) listSaveBases(c *gin.Context) {
	index, status, err := s.currentSaveIndex(c)
	if err != nil && !status.Stale {
		index = saveindex.EmptyIndex()
	}
	bases := filterBases(index.Bases, c)
	limit, offset := limitOffset(c)
	paged, summary := paginate(bases, limit, offset)
	ok(c, gin.H{"bases": flattenBases(paged), "status": status, "summary": summary})
}

func (s Server) getSaveBase(c *gin.Context) {
	index, status, err := s.currentSaveIndex(c)
	if err != nil && !status.Stale && status.State != "disabled" {
		fail(c, http.StatusServiceUnavailable, "save_index_unavailable", err.Error())
		return
	}
	id := c.Param("id")
	for _, base := range index.Bases {
		if matchesID(id, base.ID, base.Name, pallocalize.BaseName(base.Name)) {
			ok(c, gin.H{"base": flattenBase(base), "status": status})
			return
		}
	}
	fail(c, http.StatusNotFound, "base_not_found", "base not found")
}

func (s Server) getSaveBaseStorage(c *gin.Context) {
	index, status, err := s.currentSaveIndex(c)
	if err != nil && !status.Stale && status.State != "disabled" {
		fail(c, http.StatusServiceUnavailable, "save_index_unavailable", err.Error())
		return
	}
	id := c.Param("id")
	items := make([]saveindex.Container, 0)
	for _, container := range index.Containers {
		if container.OwnerType == "base" && matchesID(id, container.OwnerID) {
			items = append(items, container)
		}
	}
	ok(c, gin.H{"containers": flattenContainers(items), "status": status})
}

func (s Server) listSavePals(c *gin.Context) {
	index, status, err := s.currentSaveIndex(c)
	if err != nil && !status.Stale {
		index = saveindex.EmptyIndex()
	}
	pals := filterPals(index.Pals, index.Players, c)
	limit, offset := limitOffset(c)
	paged, summary := paginate(pals, limit, offset)
	ok(c, gin.H{"pals": flattenPals(paged, index.Players), "status": status, "summary": summary})
}

func (s Server) getSavePal(c *gin.Context) {
	index, status, err := s.currentSaveIndex(c)
	if err != nil && !status.Stale && status.State != "disabled" {
		fail(c, http.StatusServiceUnavailable, "save_index_unavailable", err.Error())
		return
	}
	id := c.Param("id")
	for _, pal := range index.Pals {
		if matchesID(id, pal.InstanceID, pal.CharacterID, pal.Nickname, pallocalize.PalName(pal.CharacterID)) {
			ok(c, gin.H{"pal": flattenPal(pal, playerLookup(index.Players)), "status": status})
			return
		}
	}
	fail(c, http.StatusNotFound, "pal_not_found", "pal not found")
}

func (s Server) listMapEntities(c *gin.Context) {
	source := strings.TrimSpace(c.Query("source"))
	if source != "" && source != "server" {
		fail(c, http.StatusBadRequest, "map_source_invalid", "source must be server when specified")
		return
	}
	manager := s.saveIndex
	if source == "server" {
		manager = s.serverSaveIndex
	}
	index, status, err := s.playerIndexCurrent(c, manager, source == "server")
	if err != nil && !status.Stale {
		index = saveindex.EmptyIndex()
	}
	online := s.onlinePlayers(c)
	status = statusWithOnlineState(status, online)
	palDefenderOnline, palDefenderAvailable, palDefenderReliable := s.palDefenderMapPlayers(c)
	online, liveSource, liveAvailable := mergeMapOnlinePlayers(online, palDefenderOnline, palDefenderAvailable, palDefenderReliable)
	entities := buildMapEntities(index, online.Players)
	limit, offset := limitOffset(c)
	paged, summary := paginate(entities, limit, offset)
	ok(c, gin.H{
		"entities": paged,
		"status":   status,
		"summary":  summary,
		"view": gin.H{
			"scope":     firstNonEmpty(source, "active"),
			"source_id": firstNonEmpty(source, "active"),
			"source_kind": func() string {
				if source == "server" {
					return "server"
				}
				return "active"
			}(),
		},
		"live": gin.H{
			"available":      liveAvailable,
			"source":         liveSource,
			"online_players": countOnlineMapEntities(entities),
			"refreshed_at":   time.Now().UTC().Format(time.RFC3339),
		},
	})
}

func (s Server) currentSaveIndex(c *gin.Context) (saveindex.Index, saveindex.Status, error) {
	// Access-driven lazy refresh: if the cache is stale relative to the save
	// files, kick off a single debounced background rebuild. This call returns
	// the current cache immediately; the next request sees the refreshed index.
	s.saveIndex.EnsureFresh(c.Request.Context())
	return s.saveIndex.Current(c.Request.Context())
}

func matchesID(needle string, candidates ...string) bool {
	needle = strings.TrimSpace(strings.ToLower(needle))
	for _, candidate := range candidates {
		if strings.ToLower(strings.TrimSpace(candidate)) == needle {
			return true
		}
	}
	return false
}

func flattenPlayers(players []saveindex.Player, online onlinePlayersResult) []gin.H {
	out := make([]gin.H, 0, len(players))
	for _, player := range players {
		out = append(out, flattenPlayer(player, online))
	}
	return out
}

func flattenPlayer(player saveindex.Player, online onlinePlayersResult) gin.H {
	metadata := onlinePlayerFor(online.Players, player)
	view := gin.H{
		"id":                firstNonEmpty(player.SteamID, player.PlayerUID),
		"player_uid":        player.PlayerUID,
		"steam_id":          firstNonEmpty(player.SteamID, player.PlayerUID),
		"nickname":          player.Nickname,
		"level":             player.Level,
		"guild_id":          player.GuildID,
		"guild_name":        pallocalize.GuildName(player.GuildName),
		"is_online":         player.IsOnline,
		"last_online_time":  player.LastOnlineTime,
		"location":          player.Location,
		"location_x":        player.Location.X,
		"location_y":        player.Location.Y,
		"location_z":        player.Location.Z,
		"x":                 player.Location.X,
		"y":                 player.Location.Y,
		"z":                 player.Location.Z,
		"inventory_summary": player.InventorySummary,
		"ping":              player.Ping,
		"ip":                player.IP,
		"online_source":     metadata.onlineSource(),
		"online_stale":      online.Stale,
	}
	if metadata.GMUserID != "" {
		view["gm_user_id"] = metadata.GMUserID
	}
	return view
}

func onlinePlayerFor(online map[string]onlinePlayer, player saveindex.Player) onlinePlayer {
	var metadata onlinePlayer
	found := false
	for _, value := range []string{player.PlayerUID, player.SteamID} {
		item, ok := online[identityKey(value)]
		if !ok {
			continue
		}
		if !found {
			metadata = item
			found = true
			continue
		}
		mergeOnlinePlayer(&metadata, item, false)
	}
	return metadata
}

func flattenBases(bases []saveindex.Base) []gin.H {
	out := make([]gin.H, 0, len(bases))
	for _, base := range bases {
		out = append(out, flattenBase(base))
	}
	return out
}

func flattenBase(base saveindex.Base) gin.H {
	palsCount := len(base.Workers)
	return gin.H{
		"id":               base.ID,
		"name":             pallocalize.BaseName(base.Name),
		"raw_name":         base.Name,
		"guild_id":         base.GuildID,
		"guild_name":       pallocalize.GuildName(base.GuildName),
		"location":         base.Location,
		"x":                base.Location.X,
		"y":                base.Location.Y,
		"z":                base.Location.Z,
		"structures_count": base.StructuresCount,
		"workers":          flattenWorkers(base.Workers),
		"containers":       base.Containers,
		"pals_count":       palsCount,
		"status":           firstNonEmpty(base.Status, "Safe"),
		"online_members":   []string{},
	}
}

func flattenPals(pals []saveindex.Pal, players []saveindex.Player) []gin.H {
	lookup := playerLookup(players)
	out := make([]gin.H, 0, len(pals))
	for _, pal := range pals {
		out = append(out, flattenPal(pal, lookup))
	}
	return out
}

func flattenPal(pal saveindex.Pal, lookup map[string]saveindex.Player) gin.H {
	owner := lookup[pal.OwnerPlayerUID]
	status := firstNonEmpty(pal.Status, "Healthy")
	speciesName := pallocalize.PalName(pal.CharacterID)
	return gin.H{
		"id":               pal.InstanceID,
		"instance_id":      pal.InstanceID,
		"character_id":     pal.CharacterID,
		"species_name":     speciesName,
		"name":             firstNonEmpty(pal.Nickname, speciesName, pal.CharacterID, pal.InstanceID),
		"nickname":         pal.Nickname,
		"level":            pal.Level,
		"rarity":           "Common",
		"rarity_name":      "普通",
		"owner_player_uid": pal.OwnerPlayerUID,
		"owner_nickname":   owner.Nickname,
		"owner_steam_id":   owner.SteamID,
		"guild_id":         firstNonEmpty(pal.GuildID, owner.GuildID),
		"container_id":     pal.ContainerID,
		"location":         pal.Location,
		"x":                pal.Location.X,
		"y":                pal.Location.Y,
		"z":                pal.Location.Z,
		"skills":           []gin.H{},
		"passives":         localizeStrings(pal.Passives, pallocalize.PassiveName),
		"raw_passives":     pal.Passives,
		"raw_skills":       pal.Skills,
		"work_suitability": []gin.H{},
		"health":           0,
		"max_health":       0,
		"status":           status,
	}
}

func flattenWorkers(workers []saveindex.Worker) []gin.H {
	out := make([]gin.H, 0, len(workers))
	for _, worker := range workers {
		speciesName := pallocalize.PalName(worker.CharacterID)
		out = append(out, gin.H{
			"instance_id":  worker.InstanceID,
			"character_id": worker.CharacterID,
			"species_name": speciesName,
			"name":         firstNonEmpty(worker.Nickname, speciesName, worker.CharacterID),
			"nickname":     worker.Nickname,
			"level":        worker.Level,
		})
	}
	return out
}

func localizeStrings(values []string, translate func(string) string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, translate(value))
	}
	return out
}

func localizeGuilds(guilds []saveindex.Guild) []saveindex.Guild {
	out := append([]saveindex.Guild(nil), guilds...)
	for i := range out {
		out[i] = localizeGuild(out[i])
	}
	return out
}

func localizeGuild(guild saveindex.Guild) saveindex.Guild {
	guild.Name = pallocalize.GuildName(guild.Name)
	return guild
}

func flattenContainers(containers []saveindex.Container) []gin.H {
	out := make([]gin.H, 0, len(containers))
	for _, container := range containers {
		slots := make([]gin.H, 0, len(container.Slots))
		for _, slot := range container.Slots {
			slots = append(slots, gin.H{
				"slot":       slot.Slot,
				"item_id":    slot.ItemID,
				"item_name":  pallocalize.ItemName(slot.ItemID),
				"count":      slot.Count,
				"durability": slot.Durability,
			})
		}
		out = append(out, gin.H{
			"container_id": container.ContainerID,
			"owner_type":   container.OwnerType,
			"owner_id":     container.OwnerID,
			"slots":        slots,
		})
	}
	return out
}

func flattenMapEntities(entities []saveindex.MapEntity) []gin.H {
	out := make([]gin.H, 0, len(entities))
	for _, entity := range entities {
		label := entity.Label
		switch entity.Type {
		case "pal":
			label = pallocalize.PalName(label)
		case "base":
			label = pallocalize.BaseName(label)
		case "map_object":
			label = pallocalize.MapObjectName(label)
		}
		out = append(out, gin.H{
			"type":      entity.Type,
			"id":        entity.ID,
			"label":     label,
			"raw_label": entity.Label,
			"location":  entity.Location,
		})
	}
	return out
}

func buildMapEntities(index saveindex.Index, online map[string]onlinePlayer) []gin.H {
	players := mergeSaveAndOnline(index.Players, online)
	entities := make([]gin.H, 0, len(players)+len(index.Bases)+len(index.Pals)+len(index.MapEntities))
	for _, player := range players {
		liveLocation := player.IsOnline && onlinePlayerHasCoordinates(online, player)
		entities = append(entities, gin.H{
			"type":       "player",
			"id":         firstNonEmpty(player.PlayerUID, player.SteamID),
			"label":      firstNonEmpty(player.Nickname, player.SteamID, player.PlayerUID, "未知玩家"),
			"location":   player.Location,
			"x":          player.Location.X,
			"y":          player.Location.Y,
			"z":          player.Location.Z,
			"is_online":  player.IsOnline,
			"live":       liveLocation,
			"source":     mapEntitySource(liveLocation),
			"guild_id":   player.GuildID,
			"guild_name": pallocalize.GuildName(player.GuildName),
			"level":      player.Level,
			"ping":       player.Ping,
		})
	}
	for _, base := range index.Bases {
		entities = append(entities, gin.H{
			"type":             "base",
			"id":               base.ID,
			"label":            pallocalize.BaseName(base.Name),
			"raw_label":        base.Name,
			"location":         base.Location,
			"x":                base.Location.X,
			"y":                base.Location.Y,
			"z":                base.Location.Z,
			"source":           "save",
			"guild_id":         base.GuildID,
			"guild_name":       pallocalize.GuildName(base.GuildName),
			"pals_count":       len(base.Workers),
			"structures_count": base.StructuresCount,
		})
	}
	for _, pal := range index.Pals {
		if pal.Location.X == 0 && pal.Location.Y == 0 && pal.Location.Z == 0 {
			continue
		}
		speciesName := pallocalize.PalName(pal.CharacterID)
		entities = append(entities, gin.H{
			"type":      "pal",
			"id":        pal.InstanceID,
			"label":     firstNonEmpty(pal.Nickname, speciesName, pal.CharacterID),
			"raw_label": firstNonEmpty(pal.Nickname, pal.CharacterID),
			"location":  pal.Location,
			"x":         pal.Location.X,
			"y":         pal.Location.Y,
			"z":         pal.Location.Z,
			"source":    "save",
			"level":     pal.Level,
			"owner_id":  pal.OwnerPlayerUID,
			"guild_id":  pal.GuildID,
		})
	}
	for _, entity := range flattenMapEntities(index.MapEntities) {
		if entity["type"] != "map_object" {
			continue
		}
		location, _ := entity["location"].(saveindex.Coordinates)
		entity["x"] = location.X
		entity["y"] = location.Y
		entity["z"] = location.Z
		entity["source"] = "save"
		entities = append(entities, entity)
	}
	return entities
}

func mapEntitySource(live bool) string {
	if live {
		return "live"
	}
	return "save"
}

func countOnlineMapEntities(entities []gin.H) int {
	count := 0
	for _, entity := range entities {
		if entity["type"] == "player" && entity["is_online"] == true {
			count++
		}
	}
	return count
}

func playerLookup(players []saveindex.Player) map[string]saveindex.Player {
	out := make(map[string]saveindex.Player, len(players)*2)
	for _, player := range players {
		out[player.PlayerUID] = player
		if player.SteamID != "" {
			out[player.SteamID] = player
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type onlinePlayersResult struct {
	Players   map[string]onlinePlayer
	Available bool
	Source    string
	Stale     bool
	Error     string
}

type palDefenderPlayersCacheEntry struct {
	Players        map[string]onlinePlayer
	Available      bool
	OnlineReliable bool
}

func normalizedPlayerKey(value string) string {
	return identityKey(value)
}

func onlinePlayerHasCoordinates(online map[string]onlinePlayer, player saveindex.Player) bool {
	for _, value := range []string{player.PlayerUID, player.SteamID} {
		if item, ok := online[normalizedPlayerKey(value)]; ok && coordinatesAvailable(item.Location) {
			return true
		}
	}
	return false
}

func coordinatesAvailable(location saveindex.Coordinates) bool {
	return location.X != 0 || location.Y != 0 || location.Z != 0
}

type onlinePlayer struct {
	PlayerUID           string
	SteamID             string
	Nickname            string
	Location            saveindex.Coordinates
	Ping                *float64
	IP                  string
	GMUserID            string
	OnlineStateKnown    bool
	IsOnline            bool
	RESTOnline          bool
	PalDefenderOnline   bool
	PalDefenderLiveData bool
}

func (p onlinePlayer) online() bool {
	if !p.OnlineStateKnown {
		return false
	}
	return p.IsOnline
}

func (p onlinePlayer) onlineSource() string {
	switch {
	case p.RESTOnline && p.PalDefenderOnline:
		return "rest+paldefender"
	case p.RESTOnline:
		return "rest"
	case p.PalDefenderOnline:
		return "paldefender"
	default:
		return "none"
	}
}

func (s Server) onlinePlayers(c *gin.Context) onlinePlayersResult {
	restPlayers, restStatus, restErr := cachedAs(s, c, cacheKey(cacheKeySavePrefix, "online-players"), 2*time.Second, func(ctx context.Context) (map[string]onlinePlayer, error) {
		resp, err := s.palworldRESTRead().Do(ctx, http.MethodGet, "players", nil)
		if err != nil {
			return nil, err
		}
		return parseOnlinePlayers(resp.Body), nil
	})
	restStale := restErr != nil || restStatus == cacheStatusStale
	if restErr != nil {
		restPlayers = map[string]onlinePlayer{}
	} else if restStale {
		restPlayers = offlineIdentityPlayers(restPlayers)
	}
	defenderPlayers, _, defenderReliable := s.palDefenderMapPlayers(c)
	if restStale {
		defenderPlayers = palDefenderPlayersForFallback(defenderPlayers, defenderReliable)
	}
	var mergedPlayers map[string]onlinePlayer
	if restStale {
		mergedPlayers = mergeOnlinePlayers(restPlayers, defenderPlayers)
	} else {
		mergedPlayers = mergeOnlinePlayersWithRESTAuthority(restPlayers, defenderPlayers)
	}
	result := onlinePlayersResult{
		Players:   mergedPlayers,
		Available: !restStale,
		Source:    "palworld_rest+paldefender",
		Stale:     restStale,
	}
	if restErr != nil {
		result.Error = restErr.Error()
	}
	return result
}

func (s Server) palDefenderMapPlayers(c *gin.Context) (map[string]onlinePlayer, bool, bool) {
	const cacheTTL = 2 * time.Second
	key := cacheKey(cacheKeySavePrefix, "paldefender-map-players")
	entry, status, err := cachedAs(s, c, key, cacheTTL, func(ctx context.Context) (palDefenderPlayersCacheEntry, error) {
		requestCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		response, err := s.defender.RESTPlayers(requestCtx)
		if err != nil {
			return palDefenderPlayersCacheEntry{}, err
		}
		players, reliable := onlinePlayersFromPalDefender(response)
		return palDefenderPlayersCacheEntry{Players: players, Available: true, OnlineReliable: reliable}, nil
	})
	if err != nil {
		entry = palDefenderPlayersCacheEntry{Players: map[string]onlinePlayer{}, Available: false, OnlineReliable: false}
		s.cache.Set(key, entry, cacheTTL)
		return entry.Players, false, false
	}
	if status == cacheStatusStale {
		entry.Players = offlineIdentityPlayers(entry.Players)
		entry.Available = false
		entry.OnlineReliable = false
		s.cache.Set(key, entry, cacheTTL)
	}
	if entry.Players == nil {
		entry.Players = map[string]onlinePlayer{}
	}
	return entry.Players, entry.Available, entry.OnlineReliable
}

func offlineIdentityPlayers(players map[string]onlinePlayer) map[string]onlinePlayer {
	out := make(map[string]onlinePlayer, len(players))
	for key, player := range players {
		player.OnlineStateKnown = true
		player.IsOnline = false
		player.RESTOnline = false
		player.PalDefenderOnline = false
		player.PalDefenderLiveData = false
		player.Location = saveindex.Coordinates{}
		player.Ping = nil
		player.IP = ""
		out[key] = player
	}
	return out
}

func onlinePlayersFromPalDefender(response paldefender.RESTPlayersResponse) (map[string]onlinePlayer, bool) {
	out := map[string]onlinePlayer{}
	onlineCount := 0
	for _, player := range response.Players {
		isOnline := strings.EqualFold(strings.TrimSpace(player.Status), "online")
		if isOnline {
			onlineCount++
		}
		location := player.WorldLocation
		if location.X == 0 && location.Y == 0 && location.Z == 0 {
			location = player.MapLocation
		}
		item := onlinePlayer{
			PlayerUID:           player.PlayerUID,
			SteamID:             player.UserID,
			Nickname:            player.Name,
			Location:            saveindex.Coordinates{X: location.X, Y: location.Y, Z: location.Z},
			IP:                  player.IP,
			GMUserID:            player.UserID,
			OnlineStateKnown:    true,
			IsOnline:            isOnline,
			PalDefenderOnline:   isOnline,
			PalDefenderLiveData: isOnline,
		}
		for _, key := range []string{player.PlayerUID, player.UserID} {
			key = normalizedPlayerKey(key)
			if key != "" {
				out[key] = item
			}
		}
	}
	reliable := response.Meta.PlayerCount == len(response.Players) && response.Meta.OnlineCount == onlineCount
	if !reliable {
		for key, player := range out {
			if !player.PalDefenderLiveData {
				continue
			}
			player.OnlineStateKnown = false
			player.IsOnline = false
			player.PalDefenderOnline = false
			out[key] = player
		}
	}
	return out, reliable
}

func palDefenderPlayersForFallback(players map[string]onlinePlayer, reliable bool) map[string]onlinePlayer {
	if reliable {
		return players
	}
	return offlineIdentityPlayers(players)
}

func mergeMapOnlinePlayers(online onlinePlayersResult, defender map[string]onlinePlayer, defenderAvailable, defenderReliable bool) (onlinePlayersResult, string, bool) {
	liveSource := online.Source
	liveAvailable := online.Available
	if !defenderAvailable {
		return online, liveSource, liveAvailable
	}
	if online.Available {
		online.Players = mergeOnlinePlayersWithRESTAuthority(online.Players, defender)
		return online, "paldefender", true
	}
	if !defenderReliable {
		return online, liveSource, liveAvailable
	}
	online.Players = mergeOnlinePlayers(online.Players, defender)
	return online, "paldefender", true
}

func mergeOnlinePlayers(primary, preferred map[string]onlinePlayer) map[string]onlinePlayer {
	registry := newOnlinePlayerRegistry()
	registry.overlay(primary, false)
	registry.overlay(preferred, true)
	return registry.players()
}

func mergeOnlinePlayersWithRESTAuthority(rest, defender map[string]onlinePlayer) map[string]onlinePlayer {
	registry := newOnlinePlayerRegistry()
	registry.overlay(rest, false)
	registry.overlay(defender, true)
	authoritative := make(map[*onlinePlayer]bool, len(rest))
	for alias, player := range rest {
		if !player.online() {
			continue
		}
		if record := registry.coalesce(alias, player.PlayerUID, player.SteamID); record != nil {
			authoritative[record] = true
		}
	}
	for _, player := range registry.records {
		if authoritative[player] {
			continue
		}
		player.OnlineStateKnown = true
		player.IsOnline = false
		player.RESTOnline = false
		player.PalDefenderOnline = false
		player.PalDefenderLiveData = false
		player.Location = saveindex.Coordinates{}
		player.Ping = nil
		player.IP = ""
	}
	return registry.players()
}

func parseOnlinePlayers(bodyValue any) map[string]onlinePlayer {
	respBody := bodyValue
	data, _ := respBody.(map[string]any)
	body := data
	if nested, ok := data["body"].(map[string]any); ok {
		body = nested
	}
	var list []any
	if raw, ok := body["players"].([]any); ok {
		list = raw
	} else if raw, ok := respBody.([]any); ok {
		list = raw
	}
	out := map[string]onlinePlayer{}
	for _, item := range list {
		player, ok := item.(map[string]any)
		if !ok {
			continue
		}
		steamID := stringFromAny(player["steam_id"], player["userId"], player["userid"])
		playerUID := stringFromAny(player["player_uid"], player["playerUid"], player["playerId"])
		nickname := stringFromAny(player["nickname"], player["name"], player["playerName"])
		pingValue, hasPing := numberFromAny(player["ping"])
		var ping *float64
		if hasPing {
			ping = &pingValue
		}
		online := onlinePlayer{
			PlayerUID:        playerUID,
			SteamID:          steamID,
			Nickname:         nickname,
			OnlineStateKnown: true,
			IsOnline:         true,
			RESTOnline:       true,
			Location: saveindex.Coordinates{
				X: numberDefault(player["location_x"], player["x"]),
				Y: numberDefault(player["location_y"], player["y"]),
				Z: numberDefault(player["location_z"], player["z"]),
			},
			Ping: ping,
			IP:   stringFromAny(player["ip"]),
		}
		for _, key := range []string{steamID, playerUID, stringFromAny(player["userId"], player["userid"])} {
			key = normalizedPlayerKey(key)
			if key != "" {
				out[key] = online
			}
		}
	}
	return out
}

func statusWithOnlineState(status saveindex.Status, online onlinePlayersResult) saveindex.Status {
	if online.Stale || online.Error != "" {
		status.Stale = true
		status.Warnings = appendUniqueString(status.Warnings, "online player REST data is stale or unavailable")
		if online.Error != "" {
			status.Warnings = appendUniqueString(status.Warnings, "online player REST error: "+online.Error)
		}
	}
	return status
}

func applyGuildOnlineCounts(guilds []saveindex.Guild, players []saveindex.Player) {
	online := make(map[string]bool, len(players)*2)
	for _, player := range players {
		if !player.IsOnline {
			continue
		}
		for _, value := range []string{player.PlayerUID, player.SteamID} {
			if key := identityKey(value); key != "" {
				online[key] = true
			}
		}
	}
	for i := range guilds {
		count := 0
		for _, member := range guilds[i].Members {
			if online[identityKey(member.PlayerUID)] {
				count++
			}
		}
		guilds[i].OnlineMemberCount = count
	}
}

func stringFromAny(values ...any) string {
	for _, value := range values {
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		case json.Number:
			return v.String()
		case float64:
			if v != 0 {
				return strings.TrimRight(strings.TrimRight(formatFloat(v), "0"), ".")
			}
		}
	}
	return ""
}

func numberDefault(values ...any) float64 {
	for _, value := range values {
		if n, ok := numberFromAny(value); ok {
			return n
		}
	}
	return 0
}

func numberFromAny(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	case string:
		if n, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

func formatFloat(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func limitOffset(c *gin.Context) (int, int) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 0 {
		limit = 0
	}
	if offset < 0 {
		offset = 0
	}
	if limit > 0 && c.Query("page") != "" && c.Query("offset") == "" {
		page, _ := strconv.Atoi(c.Query("page"))
		if page < 1 {
			page = 1
		}
		offset = (page - 1) * limit
	}
	return limit, offset
}

func paginate[T any](items []T, limit, offset int) ([]T, gin.H) {
	total := len(items)
	if offset > total {
		offset = total
	}
	end := total
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	pageItems := items[offset:end]
	pageSize := limit
	if pageSize <= 0 {
		pageSize = total
	}
	page := 1
	if pageSize > 0 {
		page = (offset / pageSize) + 1
	}
	return pageItems, gin.H{
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"returned": len(pageItems),
		"page":     page,
	}
}

func filterPlayers(players []saveindex.Player, c *gin.Context) []saveindex.Player {
	q := normalizeQuery(c.Query("q"))
	online := strings.ToLower(strings.TrimSpace(c.Query("online")))
	out := make([]saveindex.Player, 0, len(players))
	for _, player := range players {
		if q != "" && !containsAny(q, player.Nickname, player.SteamID, player.PlayerUID, player.GuildName, player.GuildID) {
			continue
		}
		if online == "true" || online == "1" {
			if !player.IsOnline {
				continue
			}
		}
		if online == "false" || online == "0" {
			if player.IsOnline {
				continue
			}
		}
		out = append(out, player)
	}
	return out
}

func filterGuilds(guilds []saveindex.Guild, c *gin.Context) []saveindex.Guild {
	q := normalizeQuery(c.Query("q"))
	if q == "" {
		return guilds
	}
	out := make([]saveindex.Guild, 0, len(guilds))
	for _, guild := range guilds {
		if containsAny(q, guild.ID, guild.Name, pallocalize.GuildName(guild.Name), guild.OwnerPlayerUID) {
			out = append(out, guild)
		}
	}
	return out
}

func filterBases(bases []saveindex.Base, c *gin.Context) []saveindex.Base {
	q := normalizeQuery(c.Query("q"))
	guildID := normalizeQuery(c.Query("guild_id"))
	out := make([]saveindex.Base, 0, len(bases))
	for _, base := range bases {
		if guildID != "" && normalizeQuery(base.GuildID) != guildID {
			continue
		}
		if q != "" && !containsAny(q, base.ID, base.Name, pallocalize.BaseName(base.Name), base.GuildName, pallocalize.GuildName(base.GuildName), base.GuildID) {
			continue
		}
		out = append(out, base)
	}
	return out
}

func filterPals(pals []saveindex.Pal, players []saveindex.Player, c *gin.Context) []saveindex.Pal {
	q := normalizeQuery(c.Query("q"))
	status := normalizeQuery(c.Query("status"))
	ownerUID := normalizeQuery(c.Query("owner_player_uid"))
	guildID := normalizeQuery(c.Query("guild_id"))
	containerID := normalizeQuery(c.Query("container_id"))
	lookup := playerLookup(players)
	out := make([]saveindex.Pal, 0, len(pals))
	for _, pal := range pals {
		owner := lookup[pal.OwnerPlayerUID]
		palGuildID := firstNonEmpty(pal.GuildID, owner.GuildID)
		if status != "" && normalizeQuery(pal.Status) != status {
			continue
		}
		if ownerUID != "" && normalizeQuery(pal.OwnerPlayerUID) != ownerUID {
			continue
		}
		if guildID != "" && normalizeQuery(palGuildID) != guildID {
			continue
		}
		if containerID != "" && normalizeQuery(pal.ContainerID) != containerID {
			continue
		}
		if q != "" && !containsAny(q, pal.InstanceID, pal.CharacterID, pallocalize.PalName(pal.CharacterID), pal.Nickname, owner.Nickname, owner.SteamID) {
			continue
		}
		out = append(out, pal)
	}
	return out
}

func normalizeQuery(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func containsAny(needle string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(normalizeQuery(value), needle) {
			return true
		}
	}
	return false
}

func appendUniqueString(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}
