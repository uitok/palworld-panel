package breeding

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"palpanel/internal/appconfig"
)

func TestCatalogLocalizesPalAndPassiveNames(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/catalog" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"version":"test","pals":[{"id":"Anubis","name":"Anubis"},{"id":"DomeArmorDragon","name":"棘甲龙","raw_name":"Aegidron"}],"passives":[{"id":"CraftSpeed_up3","name":"Artisan","supports_surgery":true},{"id":"MutationPal_Babysitter","name":"育儿专家","raw_name":"Babysitter"}],"active_skills":[]}`))
	}))
	defer upstream.Close()

	service := New(appconfig.Config{PalCalcBridgeURL: upstream.URL, PalCalcTimeoutSeconds: 5}, nil, nil)
	raw, err := service.Catalog(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Pals     []map[string]any `json:"pals"`
		Passives []map[string]any `json:"passives"`
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Pals[0]["name"] != "阿努比斯" || catalog.Pals[0]["raw_name"] != "Anubis" {
		t.Fatalf("localized pal = %#v", catalog.Pals[0])
	}
	if catalog.Passives[0]["name"] != "卓绝技艺" || catalog.Passives[0]["raw_name"] != "Artisan" {
		t.Fatalf("localized passive = %#v", catalog.Passives[0])
	}
	if catalog.Pals[1]["name"] != "棘甲龙" || catalog.Pals[1]["raw_name"] != "Aegidron" {
		t.Fatalf("upstream localized pal = %#v", catalog.Pals[1])
	}
	if catalog.Passives[1]["name"] != "育儿专家" || catalog.Passives[1]["raw_name"] != "Babysitter" {
		t.Fatalf("upstream localized passive = %#v", catalog.Passives[1])
	}
}

func TestBreedingResultKeepsUpstreamLocalizationForNewCatalogEntries(t *testing.T) {
	raw, err := localizeBreedingResult(json.RawMessage(`{"pal_id":"DomeArmorDragon","pal_name":"棘甲龙","raw_pal_name":"Aegidron","passives":["育儿专家"],"raw_passives":["MutationPal_Babysitter"]}`))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result["pal_name"] != "棘甲龙" || result["raw_pal_name"] != "Aegidron" || result["passives"].([]any)[0] != "育儿专家" || result["raw_passives"].([]any)[0] != "MutationPal_Babysitter" {
		t.Fatalf("upstream localized result = %#v", result)
	}
}

func TestBreedingResultLocalizesNestedTreesAndPreservesRawValues(t *testing.T) {
	raw, err := localizeBreedingResult(json.RawMessage(`{"results":[{"pal_id":"Anubis","pal_name":"Anubis","passives":["CraftSpeed_up3"],"tree":{"pal_id":"PinkCat","pal_name":"Cattiva","passives":["Legend"]}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	result := document["results"].([]any)[0].(map[string]any)
	if result["pal_name"] != "阿努比斯" || result["raw_pal_name"] != "Anubis" || result["passives"].([]any)[0] != "卓绝技艺" {
		t.Fatalf("localized result = %#v", result)
	}
	tree := result["tree"].(map[string]any)
	if tree["pal_name"] != "捣蛋猫" || tree["raw_pal_name"] != "Cattiva" || tree["passives"].([]any)[0] != "传说" {
		t.Fatalf("localized tree = %#v", tree)
	}
}
