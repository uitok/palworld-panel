package api

import (
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
