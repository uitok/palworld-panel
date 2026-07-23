package api

import (
	"testing"

	"palpanel/internal/saveindex"
)

func TestMergeSaveAndOnline_MatchesByUID(t *testing.T) {
	save := []saveindex.Player{{PlayerUID: "uid-1", Nickname: "存档名", Level: 10}}
	online := map[string]onlinePlayer{
		"uid-1": {PlayerUID: "uid-1", Nickname: "在线名", Location: saveindex.Coordinates{X: 5}},
	}
	got := mergeSaveAndOnline(save, indexOnline(online))
	if len(got) != 1 {
		t.Fatalf("expected 1 merged player, got %d: %#v", len(got), got)
	}
	if !got[0].IsOnline || got[0].Nickname != save[0].Nickname || got[0].Level != 10 || got[0].Location.X != 5 {
		t.Fatalf("unexpected merge result: %#v", got[0])
	}
}

func TestMergeSaveAndOnline_MatchesBySteamIDWhenSaveHasOnlyUID(t *testing.T) {
	// Save entry has both keys; online source only knows the SteamID.
	save := []saveindex.Player{{PlayerUID: "uid-1", SteamID: "steamabc", Nickname: "存档名"}}
	online := map[string]onlinePlayer{
		"steamabc": {SteamID: "steamabc"},
	}
	got := mergeSaveAndOnline(save, indexOnline(online))
	if len(got) != 1 {
		t.Fatalf("expected 1 merged player (matched by SteamID), got %d: %#v", len(got), got)
	}
	if !got[0].IsOnline || got[0].PlayerUID != "uid-1" {
		t.Fatalf("expected save identity preserved and online flag set: %#v", got[0])
	}
}

func TestMergeSaveAndOnline_ReconcilesSteamPrefix(t *testing.T) {
	save := []saveindex.Player{{PlayerUID: "uid-1", SteamID: "steam_ABC"}}
	online := map[string]onlinePlayer{
		"abc": {SteamID: "abc"},
	}
	got := mergeSaveAndOnline(save, indexOnline(online))
	if len(got) != 1 {
		t.Fatalf("expected 1 merged player (steam_ prefix reconciled), got %d: %#v", len(got), got)
	}
	if !got[0].IsOnline {
		t.Fatalf("expected online flag set after prefix reconciliation: %#v", got[0])
	}
}

func TestMergeSaveAndOnline_PalDefenderOnlyUserID(t *testing.T) {
	// PalDefender supplies only a UserID (mapped into SteamID) matching the save.
	save := []saveindex.Player{{PlayerUID: "uid-1", SteamID: "user-99", Nickname: "存档名"}}
	online := map[string]onlinePlayer{
		"user-99": {SteamID: "user-99", Location: saveindex.Coordinates{X: 1, Y: 2, Z: 3}},
	}
	got := mergeSaveAndOnline(save, indexOnline(online))
	if len(got) != 1 {
		t.Fatalf("expected 1 merged player, got %d: %#v", len(got), got)
	}
	if got[0].Location != (saveindex.Coordinates{X: 1, Y: 2, Z: 3}) {
		t.Fatalf("expected live coordinates applied: %#v", got[0])
	}
}

func TestMergeSaveAndOnline_NoDuplicateWhenOnlineKeyedTwice(t *testing.T) {
	save := []saveindex.Player{{PlayerUID: "uid-1", SteamID: "steamabc"}}
	// The same online player indexed under both keys must not duplicate.
	item := onlinePlayer{PlayerUID: "uid-1", SteamID: "steamabc"}
	online := map[string]onlinePlayer{"uid-1": item, "steamabc": item}
	got := mergeSaveAndOnline(save, indexOnline(online))
	if len(got) != 1 {
		t.Fatalf("expected 1 player (no duplicate from double-keying), got %d: %#v", len(got), got)
	}
}

func TestMergeSaveAndOnline_OnlineOnlyPlayerAppendedOnce(t *testing.T) {
	save := []saveindex.Player{{PlayerUID: "uid-save"}}
	item := onlinePlayer{PlayerUID: "uid-online", SteamID: "steam-online", Nickname: "闯入者"}
	// Online player absent from the save, indexed under both keys.
	online := map[string]onlinePlayer{"uid-online": item, "steam-online": item}
	got := mergeSaveAndOnline(save, indexOnline(online))
	if len(got) != 2 {
		t.Fatalf("expected save player + one appended online player, got %d: %#v", len(got), got)
	}
	appended := 0
	for _, p := range got {
		if p.PlayerUID == "uid-online" {
			appended++
			if !p.IsOnline || p.Nickname != "闯入者" {
				t.Fatalf("appended online player malformed: %#v", p)
			}
		}
	}
	if appended != 1 {
		t.Fatalf("expected exactly one appended online record, got %d", appended)
	}
}

func TestMergeSaveAndOnline_ThreeSourcesNoGhosts(t *testing.T) {
	// Save roster of two players; online map (already merged from REST +
	// PalDefender upstream) covers one by UID and one by SteamID.
	save := []saveindex.Player{
		{PlayerUID: "uid-1", SteamID: "steam-1", Nickname: "玩家一"},
		{PlayerUID: "uid-2", SteamID: "steam-2", Nickname: "玩家二"},
	}
	online := map[string]onlinePlayer{
		"uid-1":   {PlayerUID: "uid-1"},
		"steam-2": {SteamID: "steam-2"},
	}
	got := mergeSaveAndOnline(save, indexOnline(online))
	if len(got) != 2 {
		t.Fatalf("expected 2 players with no ghosts, got %d: %#v", len(got), got)
	}
	for _, p := range got {
		if !p.IsOnline {
			t.Fatalf("expected both matched players online: %#v", p)
		}
	}
}

func TestMergeSaveAndOnline_BridgeIdentityMergesExistingRecords(t *testing.T) {
	save := []saveindex.Player{
		{PlayerUID: "uid-bridge", Nickname: "存档玩家", Level: 30},
		{SteamID: "steam-bridge", GuildID: "guild-1", GuildName: "桥接公会"},
	}
	online := map[string]onlinePlayer{
		"uid-bridge": {
			PlayerUID: "uid-bridge",
			SteamID:   "steam-bridge",
		},
	}

	got := mergeSaveAndOnline(save, online)
	if len(got) != 1 {
		t.Fatalf("bridge identity must collapse both save records, got %d: %#v", len(got), got)
	}
	if got[0].PlayerUID != "uid-bridge" || got[0].SteamID != "steam-bridge" || got[0].Level != 30 || got[0].GuildID != "guild-1" || !got[0].IsOnline {
		t.Fatalf("bridge merge lost player data: %#v", got[0])
	}
}

func TestMergeSaveAndOnline_PreservesOverwrittenOnlineAlias(t *testing.T) {
	online := mergeOnlinePlayers(
		map[string]onlinePlayer{
			"steam_old": {
				PlayerUID:        "bridge-uid",
				SteamID:          "steam_old",
				OnlineStateKnown: true,
				IsOnline:         true,
				RESTOnline:       true,
			},
		},
		map[string]onlinePlayer{
			"steam_new": {
				PlayerUID:         "bridge-uid",
				SteamID:           "steam_new",
				GMUserID:          "steam_new",
				OnlineStateKnown:  true,
				IsOnline:          true,
				PalDefenderOnline: true,
			},
		},
	)

	got := mergeSaveAndOnline([]saveindex.Player{{
		SteamID:  "steam_old",
		Nickname: "存档玩家",
	}}, online)
	if len(got) != 1 {
		t.Fatalf("overwritten online alias created a duplicate player: %#v", got)
	}
	if !got[0].IsOnline || got[0].SteamID != "steam_new" || got[0].Nickname != "存档玩家" {
		t.Fatalf("online alias bridge lost merged state: %#v", got[0])
	}
}

func TestMergeOnlinePlayers_PalDefenderPriorityIsDeterministic(t *testing.T) {
	ping := 42.0
	primary := map[string]onlinePlayer{
		"uid-priority": {
			PlayerUID: "uid-priority",
			SteamID:   "steam-priority",
			Location:  saveindex.Coordinates{X: 1},
			Ping:      &ping,
			IP:        "rest-ip",
		},
	}
	preferred := map[string]onlinePlayer{
		"steam-priority": {
			PlayerUID: "uid-priority",
			SteamID:   "steam-priority",
			Location:  saveindex.Coordinates{X: 99},
			IP:        "paldefender-ip",
		},
	}

	merged := mergeOnlinePlayers(primary, preferred)
	for _, value := range []string{"uid-priority", "steam-priority"} {
		key := identityKey(value)
		got, ok := merged[key]
		if !ok {
			t.Fatalf("merged source is missing identity key %q: %#v", key, merged)
		}
		if got.Location.X != 99 || got.IP != "paldefender-ip" || got.Ping == nil || *got.Ping != ping {
			t.Fatalf("preferred source precedence was not preserved for %q: %#v", key, got)
		}
	}
}

// indexOnline mirrors how upstream sources register an online player under both
// its UID and SteamID keys; the test maps above already do this, so this is a
// passthrough kept for readability.
func indexOnline(online map[string]onlinePlayer) map[string]onlinePlayer {
	return online
}

func TestIdentityKeyNormalizesGUIDPunctuation(t *testing.T) {
	left := identityKey(" Steam_ABCDEF12-3456-7890-ABCD-EF1234567890 ")
	right := identityKey("abcdef1234567890abcdef1234567890")
	if left == "" || left != right {
		t.Fatalf("normalized identity mismatch: %q != %q", left, right)
	}
}

func TestIdentityKeyKeepsOnlyASCIILettersAndDigits(t *testing.T) {
	if got := identityKey("Steam_\u73a9\u5bb6-ABC_123"); got != "abc123" {
		t.Fatalf("identityKey() = %q, want %q", got, "abc123")
	}
}

func TestMergeSaveAndOnline_StaleIdentityDoesNotSetOnlineOrLiveFields(t *testing.T) {
	saveLocation := saveindex.Coordinates{X: 1, Y: 2, Z: 3}
	players := mergeSaveAndOnline([]saveindex.Player{{
		PlayerUID: "ABCDEF12-3456-7890-ABCD-EF1234567890",
		Nickname:  "存档玩家",
		Location:  saveLocation,
	}}, map[string]onlinePlayer{
		"abcdef1234567890abcdef1234567890": {
			PlayerUID:        "abcdef1234567890abcdef1234567890",
			SteamID:          "steam_76561190000000000",
			Nickname:         "陈旧 REST 名称",
			Location:         saveindex.Coordinates{X: 99, Y: 99, Z: 99},
			IP:               "203.0.113.8",
			OnlineStateKnown: true,
			IsOnline:         false,
		},
	})
	if len(players) != 1 {
		t.Fatalf("expected stale identity to merge without duplication, got %#v", players)
	}
	if players[0].IsOnline {
		t.Fatalf("stale identity must not set online: %#v", players[0])
	}
	if players[0].Location != saveLocation || players[0].IP != "" {
		t.Fatalf("stale live fields must not overwrite save data: %#v", players[0])
	}
	if players[0].SteamID != "steam_76561190000000000" {
		t.Fatalf("stale identity should still enrich identifiers: %#v", players[0])
	}
}

func TestFlattenPlayerReportsMergedOnlineSourcesAndGMIdentifier(t *testing.T) {
	rest := onlinePlayer{
		PlayerUID:  "uid-source",
		SteamID:    "steam-source",
		IsOnline:   true,
		RESTOnline: true,
	}
	defender := onlinePlayer{
		PlayerUID:         "UID-SOURCE",
		SteamID:           "steam_source",
		GMUserID:          "gm-user-1",
		IsOnline:          true,
		PalDefenderOnline: true,
	}
	online := mergeOnlinePlayers(
		map[string]onlinePlayer{"uid-source": rest},
		map[string]onlinePlayer{"steam_source": defender},
	)
	players := mergeSaveAndOnline([]saveindex.Player{{
		PlayerUID: "uid-source",
		SteamID:   "steam-source",
	}}, online)
	if len(players) != 1 {
		t.Fatalf("expected one merged player, got %#v", players)
	}
	view := flattenPlayer(players[0], onlinePlayersResult{Players: online, Stale: true})
	if view["online_source"] != "rest+paldefender" {
		t.Fatalf("unexpected online source: %#v", view)
	}
	if view["online_stale"] != true || view["gm_user_id"] != "gm-user-1" {
		t.Fatalf("missing online metadata: %#v", view)
	}
}
