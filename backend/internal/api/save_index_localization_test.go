package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"palpanel/internal/paldefender"
	"palpanel/internal/saveindex"
)

func TestSaveIndexDisplayLocalization(t *testing.T) {
	pal := saveindex.Pal{
		InstanceID:  "pal-1",
		CharacterID: "PinkCat",
		Level:       2,
		Passives:    []string{"CraftSpeed_up3", "FuturePassive"},
		Raw:         []byte{0, 255},
	}
	view := flattenPal(pal, nil)
	if _, exposed := view["raw"]; exposed {
		t.Fatalf("save-index API must not expose raw Pal payloads: %#v", view)
	}
	if view["name"] != "捣蛋猫" || view["species_name"] != "捣蛋猫" || view["character_id"] != "PinkCat" {
		t.Fatalf("Pal display identity was not localized with its raw ID preserved: %#v", view)
	}
	passives := view["passives"].([]string)
	if len(passives) != 2 || passives[0] != "卓绝技艺" || passives[1] != "FuturePassive" {
		t.Fatalf("passive names were not localized with fallback: %#v", passives)
	}

	base := flattenBase(saveindex.Base{
		Name:      "新規生成拠点テンプレート名0(仮)",
		GuildName: "Unnamed Guild",
		Workers:   []saveindex.Worker{{InstanceID: "worker-1", CharacterID: "Ganesha", Level: 4}},
		Raw:       []byte{0, 255},
	})
	if _, exposed := base["raw"]; exposed {
		t.Fatalf("save-index API must not expose raw base payloads: %#v", base)
	}
	if base["name"] != "未命名据点" || base["guild_name"] != "未命名公会" {
		t.Fatalf("base labels were not localized: %#v", base)
	}
	workers := base["workers"].([]gin.H)
	if workers[0]["name"] != "壶小象" || workers[0]["character_id"] != "Ganesha" {
		t.Fatalf("worker name was not localized: %#v", workers)
	}

	containers := flattenContainers([]saveindex.Container{{
		ContainerID: "container-1",
		Slots:       []saveindex.Slot{{Slot: 0, ItemID: "Stone", Count: 7}},
	}})
	slots := containers[0]["slots"].([]gin.H)
	if slots[0]["item_name"] != "石头" || slots[0]["item_id"] != "Stone" {
		t.Fatalf("item name was not localized: %#v", slots)
	}

	entities := flattenMapEntities([]saveindex.MapEntity{{Type: "map_object", ID: "object-1", Label: "PalBoxV2"}})
	if entities[0]["label"] != "帕鲁终端" || entities[0]["raw_label"] != "PalBoxV2" {
		t.Fatalf("map object name was not localized: %#v", entities)
	}
}

func TestFilterPalsAcceptsChineseName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/api/pals?q=捣蛋猫", nil)
	pals := []saveindex.Pal{{InstanceID: "pal-1", CharacterID: "PinkCat"}}
	if got := filterPals(pals, nil, context); len(got) != 1 {
		t.Fatalf("Chinese Pal query should match internal CharacterID: %#v", got)
	}
}

func TestOverlayOnlinePlayersUsesLiveCoordinatesWithoutDuplicates(t *testing.T) {
	location := saveindex.Coordinates{X: 123, Y: 456, Z: 789}
	live := onlinePlayer{
		PlayerUID: "player-uid",
		SteamID:   "steam-id",
		Nickname:  "在线玩家",
		Location:  location,
	}
	online := map[string]onlinePlayer{
		"player-uid": live,
		"steam-id":   live,
	}
	players := overlayOnlinePlayers([]saveindex.Player{{
		PlayerUID: "player-uid",
		SteamID:   "steam-id",
		Nickname:  "存档玩家",
		Location:  saveindex.Coordinates{X: 1, Y: 2, Z: 3},
	}}, online)
	if len(players) != 1 {
		t.Fatalf("online player was duplicated: %#v", players)
	}
	if !players[0].IsOnline || players[0].Nickname != "在线玩家" || players[0].Location != location {
		t.Fatalf("live player data did not override the save snapshot: %#v", players[0])
	}
}

func TestOverlayOnlinePlayersMatchesSteamPrefixVariants(t *testing.T) {
	live := onlinePlayer{
		PlayerUID: "live-player-uid",
		SteamID:   "steam_76561198370732375",
		Nickname:  "玛卡巴卡",
	}
	online := map[string]onlinePlayer{
		normalizedPlayerKey(live.PlayerUID): live,
		normalizedPlayerKey(live.SteamID):   live,
	}
	players := overlayOnlinePlayers([]saveindex.Player{{
		PlayerUID: "save-player-uid",
		SteamID:   "76561198370732375",
		Nickname:  "玛卡巴卡",
		Level:     37,
	}}, online)

	if len(players) != 1 {
		t.Fatalf("Steam prefix variant created a duplicate player: %#v", players)
	}
	if !players[0].IsOnline || players[0].Level != 37 || players[0].SteamID != live.SteamID {
		t.Fatalf("live identity was not overlaid onto the save player: %#v", players[0])
	}
}

func TestOverlayOnlinePlayersPreservesSaveCoordinatesWhenRESTHasNone(t *testing.T) {
	saveLocation := saveindex.Coordinates{X: 11, Y: 22, Z: 33}
	players := overlayOnlinePlayers([]saveindex.Player{{
		PlayerUID: "player-uid",
		SteamID:   "steam-id",
		Location:  saveLocation,
	}}, map[string]onlinePlayer{
		"player-uid": {PlayerUID: "player-uid", SteamID: "steam-id", Nickname: "在线玩家"},
	})
	if len(players) != 1 || !players[0].IsOnline || players[0].Location != saveLocation {
		t.Fatalf("REST data without coordinates replaced the save position: %#v", players)
	}
}

func TestOnlinePlayersFromPalDefenderPrefersWorldLocation(t *testing.T) {
	players := onlinePlayersFromPalDefender(paldefender.RESTPlayersResponse{Players: []paldefender.RESTPlayer{
		{
			Name:          "World",
			PlayerUID:     "world-uid",
			UserID:        "world-steam",
			Status:        "Online",
			WorldLocation: paldefender.RESTLocation{X: 10, Y: 20, Z: 30},
			MapLocation:   paldefender.RESTLocation{X: 100, Y: 200, Z: 300},
		},
		{
			Name:        "Map fallback",
			PlayerUID:   "map-uid",
			UserID:      "map-steam",
			Status:      "online",
			MapLocation: paldefender.RESTLocation{X: 40, Y: 50, Z: 60},
		},
		{Name: "Offline", PlayerUID: "offline-uid", Status: "offline"},
	}})
	if len(players) != 4 {
		t.Fatalf("expected two identities for each online player, got %#v", players)
	}
	if got := players["world-uid"].Location; got != (saveindex.Coordinates{X: 10, Y: 20, Z: 30}) {
		t.Fatalf("world location was not preferred: %#v", got)
	}
	if got := players["map-steam"].Location; got != (saveindex.Coordinates{X: 40, Y: 50, Z: 60}) {
		t.Fatalf("map location fallback was not used: %#v", got)
	}
	if _, ok := players["offline-uid"]; ok {
		t.Fatal("offline PalDefender player should not be exposed as live")
	}
}

func TestBuildMapEntitiesPreservesStaticEntities(t *testing.T) {
	index := saveindex.Index{
		Players: []saveindex.Player{{PlayerUID: "player", Nickname: "玩家", Location: saveindex.Coordinates{X: 1}}},
		Bases:   []saveindex.Base{{ID: "base", Name: "基地", Location: saveindex.Coordinates{X: 2}}},
		Pals:    []saveindex.Pal{{InstanceID: "pal", CharacterID: "PinkCat", Location: saveindex.Coordinates{X: 3}}},
		MapEntities: []saveindex.MapEntity{{
			Type: "map_object", ID: "object", Label: "PalBoxV2", Location: saveindex.Coordinates{X: 4},
		}},
	}
	entities := buildMapEntities(index, nil)
	if len(entities) != 4 {
		t.Fatalf("static map entities were lost: %#v", entities)
	}
	for _, entity := range entities {
		if entity["source"] != "save" {
			t.Fatalf("unexpected static entity source: %#v", entity)
		}
	}
}

func TestSaveIndexQueryHelpersAndFilters(t *testing.T) {
	if got := stringFromAny("  ", json.Number("42")); got != "42" {
		t.Fatalf("stringFromAny json number = %q", got)
	}
	if got := stringFromAny(float64(12.5)); got != "12.5" {
		t.Fatalf("stringFromAny float = %q", got)
	}
	if got := numberDefault("bad", float32(2.5)); got != 2.5 {
		t.Fatalf("numberDefault = %v", got)
	}
	for _, test := range []struct {
		value any
		want  float64
		ok    bool
	}{
		{float64(1), 1, true}, {float32(2), 2, true}, {3, 3, true}, {int64(4), 4, true},
		{json.Number("5.5"), 5.5, true}, {" 6.5 ", 6.5, true}, {"bad", 0, false}, {struct{}{}, 0, false},
	} {
		got, ok := numberFromAny(test.value)
		if got != test.want || ok != test.ok {
			t.Errorf("numberFromAny(%#v) = %v, %v", test.value, got, ok)
		}
	}

	newQueryContext := func(target string) *gin.Context {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest("GET", target, nil)
		return context
	}
	context := newQueryContext("/api/players?limit=2&page=3&offset=-1")
	if limit, offset := limitOffset(context); limit != 2 || offset != 0 {
		t.Fatalf("limitOffset explicit offset = %d, %d", limit, offset)
	}
	context = newQueryContext("/api/players?limit=2&page=3")
	if limit, offset := limitOffset(context); limit != 2 || offset != 4 {
		t.Fatalf("limitOffset page = %d, %d", limit, offset)
	}
	page, meta := paginate([]int{1, 2, 3, 4, 5}, 2, 2)
	if len(page) != 2 || page[0] != 3 || meta["page"] != 2 {
		t.Fatalf("paginate = %#v %#v", page, meta)
	}
	page, meta = paginate([]int{1, 2}, 0, 99)
	if len(page) != 0 || meta["offset"] != 2 {
		t.Fatalf("paginate clamped offset = %#v %#v", page, meta)
	}

	status := statusWithOnlineState(saveindex.Status{}, onlinePlayersResult{Stale: true, Error: "offline"})
	if !status.Stale || len(status.Warnings) != 2 {
		t.Fatalf("statusWithOnlineState = %#v", status)
	}
	if got := appendUniqueString(status.Warnings, status.Warnings[0]); len(got) != 2 {
		t.Fatalf("appendUniqueString duplicated value: %#v", got)
	}

	guilds := []saveindex.Guild{{ID: "guild-1", Name: "Guild", Members: []saveindex.GuildMember{{PlayerUID: "online"}, {PlayerUID: "offline"}}}}
	applyGuildOnlineCounts(guilds, map[string]onlinePlayer{"online": {PlayerUID: "online"}})
	if guilds[0].OnlineMemberCount != 1 {
		t.Fatalf("applyGuildOnlineCounts = %#v", guilds)
	}

	players := []saveindex.Player{
		{PlayerUID: "uid-1", SteamID: "steam-1", Nickname: "Online", GuildID: "guild-1", GuildName: "Guild", IsOnline: true},
		{PlayerUID: "uid-2", SteamID: "steam-2", Nickname: "Offline", IsOnline: false},
	}
	context = newQueryContext("/api/players?q=online&online=true")
	if got := filterPlayers(players, context); len(got) != 1 || got[0].PlayerUID != "uid-1" {
		t.Fatalf("filterPlayers online = %#v", got)
	}
	context = newQueryContext("/api/guilds?q=guild-1")
	if got := filterGuilds(guilds, context); len(got) != 1 {
		t.Fatalf("filterGuilds = %#v", got)
	}
	bases := []saveindex.Base{{ID: "base-1", Name: "Main", GuildID: "guild-1"}, {ID: "base-2", Name: "Other", GuildID: "guild-2"}}
	context = newQueryContext("/api/bases?guild_id=guild-1&q=main")
	if got := filterBases(bases, context); len(got) != 1 || got[0].ID != "base-1" {
		t.Fatalf("filterBases = %#v", got)
	}
	pals := []saveindex.Pal{
		{InstanceID: "pal-1", CharacterID: "Anubis", OwnerPlayerUID: "uid-1", GuildID: "guild-1", ContainerID: "box-1", Status: "stored"},
		{InstanceID: "pal-2", CharacterID: "PinkCat", OwnerPlayerUID: "uid-2", GuildID: "guild-2", ContainerID: "box-2", Status: "party"},
	}
	context = newQueryContext("/api/pals?q=anubis&status=stored&owner_player_uid=uid-1&guild_id=guild-1&container_id=box-1")
	if got := filterPals(pals, players, context); len(got) != 1 || got[0].InstanceID != "pal-1" {
		t.Fatalf("filterPals = %#v", got)
	}
}
