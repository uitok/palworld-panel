package api

import (
	"strings"

	"palpanel/internal/saveindex"
)

// playeridentity.go centralizes reconciliation of the same physical player
// across the three data sources the panel merges:
//
//   - the save parse (saveindex.Player) — the authoritative offline roster
//     (nickname, level, guild, last-online time, save-derived location);
//   - the Palworld official REST /players list — live online flag, ping, IP,
//     live coordinates;
//   - the PalDefender anti-cheat REST list — live coordinates preferred over
//     Palworld REST, plus IP.
//
// Each source names identity fields differently (save: player_uid/steam_id;
// Palworld REST: userId/playerId/steam_id; PalDefender: PlayerUID/UserId), so a
// player can hold a UID in one source and only a SteamID in another. Matching on
// a single key therefore produced duplicate "ghost" entries. The registry links
// entries that share *either* a UID or a SteamID (transitively), guaranteeing one
// merged record per physical player regardless of which key each source carries.

// identityKey normalizes a UID or SteamID into a comparison key: trimmed,
// lowercased, stripped to ASCII letters and digits, and with the ubiquitous
// "steam_" prefix removed so that "steam_abc" and "abc" reconcile.
func identityKey(value string) string {
	key := strings.ToLower(strings.TrimSpace(value))
	if strings.HasPrefix(key, "steam_") && len(key) > len("steam_") {
		key = key[len("steam_"):]
	}
	var normalized strings.Builder
	for index := 0; index < len(key); index++ {
		char := key[index]
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			normalized.WriteByte(char)
		}
	}
	return normalized.String()
}

// playerRegistry merges save and live player records by identity, linking any
// entries that share a UID or SteamID.
type playerRegistry struct {
	records []*saveindex.Player
	// byKey maps every normalized identity key (UID or SteamID) a record is
	// known by to that record, so a later source matches on any shared key.
	byKey map[string]*saveindex.Player
}

func newPlayerRegistry(base []saveindex.Player) *playerRegistry {
	r := &playerRegistry{
		records: make([]*saveindex.Player, 0, len(base)),
		byKey:   make(map[string]*saveindex.Player, len(base)*2),
	}
	for i := range base {
		player := base[i]
		player.IsOnline = false
		if existing := r.coalesce(player.PlayerUID, player.SteamID); existing != nil {
			mergeSavePlayer(existing, &player)
			r.index(existing)
		} else {
			r.add(&player)
		}
	}
	return r
}

// add appends a new record and indexes all of its identity keys.
func (r *playerRegistry) add(player *saveindex.Player, aliases ...string) {
	r.records = append(r.records, player)
	r.index(player, aliases...)
}

// index (re)registers every identity key a record currently carries.
func (r *playerRegistry) index(player *saveindex.Player, aliases ...string) {
	values := append(append([]string(nil), aliases...), player.PlayerUID, player.SteamID)
	for _, value := range values {
		if key := identityKey(value); key != "" {
			r.byKey[key] = player
		}
	}
}

// lookup finds an existing record that shares any identity key.
func (r *playerRegistry) lookupAll(values ...string) []*saveindex.Player {
	matches := make([]*saveindex.Player, 0, len(values))
	seen := make(map[*saveindex.Player]bool, len(values))
	for _, value := range values {
		if key := identityKey(value); key != "" {
			if existing, ok := r.byKey[key]; ok && !seen[existing] {
				seen[existing] = true
				matches = append(matches, existing)
			}
		}
	}
	return matches
}

func (r *playerRegistry) coalesce(values ...string) *saveindex.Player {
	matches := r.lookupAll(values...)
	if len(matches) == 0 {
		return nil
	}
	canonical := matches[0]
	for _, duplicate := range matches[1:] {
		mergeSavePlayer(canonical, duplicate)
		for key, record := range r.byKey {
			if record == duplicate {
				r.byKey[key] = canonical
			}
		}
		for i, record := range r.records {
			if record == duplicate {
				r.records = append(r.records[:i], r.records[i+1:]...)
				break
			}
		}
	}
	r.index(canonical)
	return canonical
}

func mergeSavePlayer(target, source *saveindex.Player) {
	if target.PlayerUID == "" {
		target.PlayerUID = source.PlayerUID
	}
	if target.SteamID == "" {
		target.SteamID = source.SteamID
	}
	if target.Nickname == "" {
		target.Nickname = source.Nickname
	}
	if target.Level == 0 {
		target.Level = source.Level
	}
	if target.GuildID == "" {
		target.GuildID = source.GuildID
	}
	if target.GuildName == "" {
		target.GuildName = source.GuildName
	}
	target.IsOnline = target.IsOnline || source.IsOnline
	if target.LastOnlineTime == "" {
		target.LastOnlineTime = source.LastOnlineTime
	}
	if !coordinatesAvailable(target.Location) {
		target.Location = source.Location
	}
	if target.InventorySummary == nil {
		target.InventorySummary = source.InventorySummary
	}
	if target.Ping == nil {
		target.Ping = source.Ping
	}
	if target.IP == "" {
		target.IP = source.IP
	}
	if target.Raw == nil {
		target.Raw = source.Raw
	}
}

// overlayOnline merges an online-player map (from Palworld REST or PalDefender)
// onto the registry. Live fields (online flag, ping, IP, coordinates) win from
// the online source; identity is preserved from the save when present. Online
// players absent from the save are appended once as standalone online records.
func (r *playerRegistry) overlayOnline(online map[string]onlinePlayer) {
	// An online source may key the same record under both its UID and SteamID,
	// so the map can contain duplicate entries for one physical player. We track
	// which online identities we've already appended to avoid creating two
	// standalone records for the same player.
	appendedKeys := make(map[string]bool)
	for alias, item := range online {
		if strings.TrimSpace(item.PlayerUID) == "" && strings.TrimSpace(item.SteamID) == "" {
			continue
		}
		if existing := r.coalesce(alias, item.PlayerUID, item.SteamID); existing != nil {
			applyOnlineToPlayer(existing, item)
			// The online source may carry an identity key the save lacked;
			// index it so a later entry reconciles to this same record.
			r.index(existing, alias)
			continue
		}
		if onlineAlreadyAppended(appendedKeys, item, alias) {
			continue
		}
		appended := saveindex.Player{
			PlayerUID: item.PlayerUID,
			SteamID:   item.SteamID,
			Nickname:  item.Nickname,
			IsOnline:  item.online(),
		}
		if item.online() {
			appended.Location = item.Location
			appended.Ping = item.Ping
			appended.IP = item.IP
		}
		r.add(&appended, alias)
		markOnlineAppended(appendedKeys, item, alias)
	}
}

func onlineAlreadyAppended(appended map[string]bool, item onlinePlayer, aliases ...string) bool {
	values := append(append([]string(nil), aliases...), item.PlayerUID, item.SteamID)
	for _, value := range values {
		if key := identityKey(value); key != "" && appended[key] {
			return true
		}
	}
	return false
}

func markOnlineAppended(appended map[string]bool, item onlinePlayer, aliases ...string) {
	values := append(append([]string(nil), aliases...), item.PlayerUID, item.SteamID)
	for _, value := range values {
		if key := identityKey(value); key != "" {
			appended[key] = true
		}
	}
}

// applyOnlineToPlayer copies live fields from an online record onto a merged
// player, matching the field precedence the panel relied on previously.
func applyOnlineToPlayer(player *saveindex.Player, item onlinePlayer) {
	if item.SteamID != "" {
		player.SteamID = item.SteamID
	}
	if item.PlayerUID != "" {
		player.PlayerUID = item.PlayerUID
	}
	if player.Nickname == "" && item.Nickname != "" {
		player.Nickname = item.Nickname
	}
	if !item.online() {
		return
	}
	player.IsOnline = true
	player.Ping = item.Ping
	player.IP = item.IP
	if coordinatesAvailable(item.Location) {
		player.Location = item.Location
	}
}

// players returns the merged roster.
func (r *playerRegistry) players() []saveindex.Player {
	out := make([]saveindex.Player, 0, len(r.records))
	for _, record := range r.records {
		out = append(out, *record)
	}
	return out
}

// mergeSaveAndOnline is the single entry point that reconciles the save roster
// with an online map into one deduplicated slice.
func mergeSaveAndOnline(save []saveindex.Player, online map[string]onlinePlayer) []saveindex.Player {
	registry := newPlayerRegistry(save)
	registry.overlayOnline(online)
	return registry.players()
}

type onlinePlayerRegistry struct {
	records []*onlinePlayer
	byKey   map[string]*onlinePlayer
}

func newOnlinePlayerRegistry() *onlinePlayerRegistry {
	return &onlinePlayerRegistry{byKey: make(map[string]*onlinePlayer)}
}

func (r *onlinePlayerRegistry) overlay(source map[string]onlinePlayer, preferred bool) {
	for alias, item := range source {
		existing := r.coalesce(alias, item.PlayerUID, item.SteamID)
		if existing == nil {
			copy := item
			r.records = append(r.records, &copy)
			existing = &copy
		}
		mergeOnlinePlayer(existing, item, preferred)
		r.index(existing, alias)
	}
}

func (r *onlinePlayerRegistry) coalesce(values ...string) *onlinePlayer {
	matches := make([]*onlinePlayer, 0, len(values))
	seen := make(map[*onlinePlayer]bool, len(values))
	for _, value := range values {
		if key := identityKey(value); key != "" {
			if record := r.byKey[key]; record != nil && !seen[record] {
				seen[record] = true
				matches = append(matches, record)
			}
		}
	}
	if len(matches) == 0 {
		return nil
	}
	canonical := matches[0]
	for _, duplicate := range matches[1:] {
		mergeOnlinePlayer(canonical, *duplicate, false)
		for key, record := range r.byKey {
			if record == duplicate {
				r.byKey[key] = canonical
			}
		}
		for i, record := range r.records {
			if record == duplicate {
				r.records = append(r.records[:i], r.records[i+1:]...)
				break
			}
		}
	}
	return canonical
}

func (r *onlinePlayerRegistry) index(player *onlinePlayer, aliases ...string) {
	for _, value := range append(aliases, player.PlayerUID, player.SteamID) {
		if key := identityKey(value); key != "" {
			r.byKey[key] = player
		}
	}
}

func (r *onlinePlayerRegistry) players() map[string]onlinePlayer {
	out := make(map[string]onlinePlayer, len(r.byKey))
	for key, player := range r.byKey {
		out[key] = *player
	}
	return out
}

func mergeOnlinePlayer(target *onlinePlayer, source onlinePlayer, preferred bool) {
	targetOnline := target.online()
	sourceOnline := source.online()
	if target.PlayerUID == "" || preferred && source.PlayerUID != "" {
		target.PlayerUID = source.PlayerUID
	}
	if target.SteamID == "" || preferred && source.SteamID != "" {
		target.SteamID = source.SteamID
	}
	if target.Nickname == "" || preferred && source.Nickname != "" {
		target.Nickname = source.Nickname
	}
	if target.GMUserID == "" || preferred && source.GMUserID != "" {
		target.GMUserID = source.GMUserID
	}
	target.OnlineStateKnown = target.OnlineStateKnown || source.OnlineStateKnown
	target.IsOnline = targetOnline || sourceOnline
	target.RESTOnline = target.RESTOnline || source.RESTOnline
	target.PalDefenderOnline = target.PalDefenderOnline || source.PalDefenderOnline
	if !sourceOnline {
		return
	}
	if !coordinatesAvailable(target.Location) || preferred && coordinatesAvailable(source.Location) {
		if coordinatesAvailable(source.Location) {
			target.Location = source.Location
		}
	}
	if target.Ping == nil || preferred && source.Ping != nil {
		if source.Ping != nil {
			target.Ping = source.Ping
		}
	}
	if target.IP == "" || preferred && source.IP != "" {
		if source.IP != "" {
			target.IP = source.IP
		}
	}
}
