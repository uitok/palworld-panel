package indexer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLiveOneDotZeroFixture(t *testing.T) {
	path := os.Getenv("PAL_LIVE_FIXTURE")
	if path == "" {
		t.Skip("PAL_LIVE_FIXTURE is not set")
	}
	if filepath.Base(path) == "Level.sav" {
		path = filepath.Dir(path)
	}

	index, err := Build(path)
	if err != nil {
		t.Fatal(err)
	}
	if index.Counts.Players != 1 || index.Counts.Guilds != 1 || index.Counts.Bases != 1 || index.Counts.Pals != 1 || index.Counts.Containers != 71 {
		t.Fatalf("unexpected live 1.0 counts: %#v", index.Counts)
	}
	if len(index.Warnings) != 0 {
		t.Fatalf("live 1.0 fixture produced warnings: %#v", index.Warnings)
	}
	if index.Players[0].Level != 3 {
		t.Fatalf("expected player level 3, got %d", index.Players[0].Level)
	}
	if index.Pals[0].CharacterID != "PinkCat" || index.Pals[0].Level != 2 || index.Pals[0].GuildID == "" {
		t.Fatalf("captured Pal was not indexed: %#v", index.Pals[0])
	}
	if len(index.Bases[0].Workers) != 1 || index.Bases[0].Workers[0].InstanceID != index.Pals[0].InstanceID {
		t.Fatalf("base worker relationship was not indexed: %#v", index.Bases[0].Workers)
	}
	if !hasMapLabel(index.MapEntities, "PalBoxV2") {
		t.Fatalf("PalBoxV2 was not indexed: %#v", index.MapEntities)
	}
}

func hasMapLabel(entities []MapEntity, label string) bool {
	for _, entity := range entities {
		if entity.Label == label {
			return true
		}
	}
	return false
}
