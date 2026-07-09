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

	"palpanel/internal/saveindex"
)

func (s Server) saveIndexStatus(c *gin.Context) {
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
	ok(c, gin.H{"status": status, "index": index})
}

func (s Server) listSavePlayers(c *gin.Context) {
	index, status, err := s.currentSaveIndex(c)
	if err != nil && !status.Stale {
		index = saveindex.EmptyIndex()
	}
	online := s.onlinePlayers(c)
	status = statusWithOnlineState(status, online)
	players := overlayOnlinePlayers(index.Players, online.Players)
	players = filterPlayers(players, c)
	limit, offset := limitOffset(c)
	paged, summary := paginate(players, limit, offset)
	ok(c, gin.H{"players": flattenPlayers(paged), "status": status, "summary": summary})
}

func (s Server) getSavePlayer(c *gin.Context) {
	index, status, err := s.currentSaveIndex(c)
	if err != nil && !status.Stale && status.State != "disabled" {
		fail(c, http.StatusServiceUnavailable, "save_index_unavailable", err.Error())
		return
	}
	id := c.Param("id")
	players := overlayOnlinePlayers(index.Players, s.onlinePlayers(c).Players)
	for _, player := range players {
		if matchesID(id, player.PlayerUID, player.SteamID) {
			ok(c, gin.H{"player": flattenPlayer(player), "status": status})
			return
		}
	}
	fail(c, http.StatusNotFound, "player_not_found", "player not found")
}

func (s Server) getSavePlayerInventory(c *gin.Context) {
	index, status, err := s.currentSaveIndex(c)
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
	ok(c, gin.H{"containers": items, "status": status})
}

func (s Server) listSaveGuilds(c *gin.Context) {
	index, status, err := s.currentSaveIndex(c)
	if err != nil && !status.Stale {
		index = saveindex.EmptyIndex()
	}
	online := s.onlinePlayers(c)
	status = statusWithOnlineState(status, online)
	guilds := append([]saveindex.Guild(nil), index.Guilds...)
	applyGuildOnlineCounts(guilds, online.Players)
	guilds = filterGuilds(guilds, c)
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
	applyGuildOnlineCounts(guilds, s.onlinePlayers(c).Players)
	id := c.Param("id")
	for _, guild := range guilds {
		if matchesID(id, guild.ID, guild.OwnerPlayerUID, guild.Name) {
			ok(c, gin.H{"guild": guild, "status": status})
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
		if matchesID(id, base.ID, base.Name) {
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
	ok(c, gin.H{"containers": items, "status": status})
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
		if matchesID(id, pal.InstanceID, pal.CharacterID, pal.Nickname) {
			ok(c, gin.H{"pal": flattenPal(pal, playerLookup(index.Players)), "status": status})
			return
		}
	}
	fail(c, http.StatusNotFound, "pal_not_found", "pal not found")
}

func (s Server) listMapEntities(c *gin.Context) {
	index, status, err := s.currentSaveIndex(c)
	if err != nil && !status.Stale {
		index = saveindex.EmptyIndex()
	}
	limit, offset := limitOffset(c)
	paged, summary := paginate(index.MapEntities, limit, offset)
	ok(c, gin.H{"entities": paged, "status": status, "summary": summary})
}

func (s Server) currentSaveIndex(c *gin.Context) (saveindex.Index, saveindex.Status, error) {
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

func flattenPlayers(players []saveindex.Player) []gin.H {
	out := make([]gin.H, 0, len(players))
	for _, player := range players {
		out = append(out, flattenPlayer(player))
	}
	return out
}

func flattenPlayer(player saveindex.Player) gin.H {
	return gin.H{
		"id":                firstNonEmpty(player.SteamID, player.PlayerUID),
		"player_uid":        player.PlayerUID,
		"steam_id":          firstNonEmpty(player.SteamID, player.PlayerUID),
		"nickname":          player.Nickname,
		"level":             player.Level,
		"guild_id":          player.GuildID,
		"guild_name":        player.GuildName,
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
		"raw":               player.Raw,
	}
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
		"name":             base.Name,
		"guild_id":         base.GuildID,
		"guild_name":       base.GuildName,
		"location":         base.Location,
		"x":                base.Location.X,
		"y":                base.Location.Y,
		"z":                base.Location.Z,
		"structures_count": base.StructuresCount,
		"workers":          base.Workers,
		"containers":       base.Containers,
		"pals_count":       palsCount,
		"status":           firstNonEmpty(base.Status, "Safe"),
		"online_members":   []string{},
		"raw":              base.Raw,
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
	return gin.H{
		"id":               pal.InstanceID,
		"instance_id":      pal.InstanceID,
		"character_id":     pal.CharacterID,
		"name":             firstNonEmpty(pal.Nickname, pal.CharacterID, pal.InstanceID),
		"nickname":         pal.Nickname,
		"level":            pal.Level,
		"rarity":           "Common",
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
		"passives":         pal.Passives,
		"raw_skills":       pal.Skills,
		"work_suitability": []gin.H{},
		"health":           0,
		"max_health":       0,
		"status":           status,
		"raw":              pal.Raw,
	}
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
	Players map[string]onlinePlayer
	Stale   bool
	Error   string
}

func overlayOnlinePlayers(players []saveindex.Player, online map[string]onlinePlayer) []saveindex.Player {
	out := append([]saveindex.Player(nil), players...)
	seen := map[string]bool{}
	for i := range out {
		for _, key := range []string{out[i].PlayerUID, out[i].SteamID} {
			if item, ok := online[strings.ToLower(key)]; ok {
				out[i].IsOnline = true
				out[i].Ping = item.Ping
				out[i].IP = item.IP
				if item.Nickname != "" {
					out[i].Nickname = item.Nickname
				}
				if item.SteamID != "" {
					out[i].SteamID = item.SteamID
				}
				if item.PlayerUID != "" {
					out[i].PlayerUID = item.PlayerUID
				}
				out[i].Location = item.Location
				seen[strings.ToLower(key)] = true
			}
		}
	}
	for key, item := range online {
		if seen[key] || item.SteamID == "" {
			continue
		}
		out = append(out, saveindex.Player{
			PlayerUID: item.PlayerUID,
			SteamID:   item.SteamID,
			Nickname:  item.Nickname,
			IsOnline:  true,
			Location:  item.Location,
			Ping:      item.Ping,
			IP:        item.IP,
		})
	}
	return out
}

type onlinePlayer struct {
	PlayerUID string
	SteamID   string
	Nickname  string
	Location  saveindex.Coordinates
	Ping      *float64
	IP        string
}

func (s Server) onlinePlayers(c *gin.Context) onlinePlayersResult {
	players, status, err := cachedAs(s, c, cacheKey(cacheKeySavePrefix, "online-players"), 2*time.Second, func(ctx context.Context) (map[string]onlinePlayer, error) {
		resp, err := s.palworldRESTRead().Do(ctx, http.MethodGet, "players", nil)
		if err != nil {
			return nil, err
		}
		return parseOnlinePlayers(resp.Body), nil
	})
	if err != nil {
		return onlinePlayersResult{Players: map[string]onlinePlayer{}, Stale: true, Error: err.Error()}
	}
	return onlinePlayersResult{Players: players, Stale: status == cacheStatusStale}
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
		steamID := stringFromAny(player["steam_id"], player["userId"], player["userid"], player["playerId"])
		playerUID := stringFromAny(player["player_uid"], player["playerUid"])
		nickname := stringFromAny(player["nickname"], player["name"], player["playerName"])
		pingValue, hasPing := numberFromAny(player["ping"])
		var ping *float64
		if hasPing {
			ping = &pingValue
		}
		online := onlinePlayer{
			PlayerUID: playerUID,
			SteamID:   steamID,
			Nickname:  nickname,
			Location: saveindex.Coordinates{
				X: numberDefault(player["location_x"], player["x"]),
				Y: numberDefault(player["location_y"], player["y"]),
				Z: numberDefault(player["location_z"], player["z"]),
			},
			Ping: ping,
			IP:   stringFromAny(player["ip"]),
		}
		for _, key := range []string{steamID, playerUID, stringFromAny(player["userId"], player["userid"])} {
			key = strings.ToLower(strings.TrimSpace(key))
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

func applyGuildOnlineCounts(guilds []saveindex.Guild, online map[string]onlinePlayer) {
	for i := range guilds {
		count := 0
		for _, member := range guilds[i].Members {
			if _, ok := online[strings.ToLower(member.PlayerUID)]; ok {
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
		if containsAny(q, guild.ID, guild.Name, guild.OwnerPlayerUID) {
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
		if q != "" && !containsAny(q, base.ID, base.Name, base.GuildName, base.GuildID) {
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
		if q != "" && !containsAny(q, pal.InstanceID, pal.CharacterID, pal.Nickname, owner.Nickname, owner.SteamID) {
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
