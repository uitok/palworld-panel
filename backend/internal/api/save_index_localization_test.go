package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"palpanel/internal/saveindex"
)

func TestSaveIndexDisplayLocalization(t *testing.T) {
	pal := saveindex.Pal{
		InstanceID:  "pal-1",
		CharacterID: "PinkCat",
		Level:       2,
		Passives:    []string{"CraftSpeed_up3", "FuturePassive"},
	}
	view := flattenPal(pal, nil)
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
	})
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
