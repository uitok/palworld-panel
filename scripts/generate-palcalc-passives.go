package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type palCalcDatabase struct {
	Version       string `json:"Version"`
	PassiveSkills []struct {
		InternalName           string            `json:"InternalName"`
		LocalizedNames         map[string]string `json:"LocalizedNames"`
		IsStandardPassiveSkill bool              `json:"IsStandardPassiveSkill"`
	} `json:"PassiveSkills"`
}

type sourceInfo struct {
	Project string `json:"project"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	License string `json:"license"`
}

type outputCatalog struct {
	Source        json.RawMessage   `json:"source"`
	PassiveSource sourceInfo        `json:"passives_source"`
	Pals          json.RawMessage   `json:"pals"`
	Items         json.RawMessage   `json:"items"`
	Passives      map[string]string `json:"passives"`
	ItemIcons     json.RawMessage   `json:"item_icons"`
}

func main() {
	databasePath := flag.String("db", filepath.FromSlash("third_party/palcalc/PalCalc.Model/db.json"), "PalCalc db.json path")
	catalogPath := flag.String("catalog", filepath.FromSlash("backend/internal/pallocalize/catalog.zh-CN.json"), "PalPanel catalog path")
	commit := flag.String("commit", "8b7e2f779e47fddae16ddcb973e828ba20c02b80", "pinned PalCalc commit")
	flag.Parse()

	databasePayload, err := os.ReadFile(*databasePath)
	check(err)
	var database palCalcDatabase
	check(json.Unmarshal(databasePayload, &database))
	if strings.TrimSpace(database.Version) == "" {
		check(fmt.Errorf("PalCalc database version is empty"))
	}

	passives := map[string]string{}
	for _, passive := range database.PassiveSkills {
		if !passive.IsStandardPassiveSkill {
			continue
		}
		id := strings.TrimSpace(passive.InternalName)
		name := strings.TrimSpace(passive.LocalizedNames["zh-Hans"])
		if id == "" || name == "" {
			check(fmt.Errorf("standard passive is missing an ID or zh-Hans name: %q", id))
		}
		if _, exists := passives[id]; exists {
			check(fmt.Errorf("duplicate standard passive ID %q", id))
		}
		passives[id] = name
	}
	if len(passives) != 115 {
		check(fmt.Errorf("expected 115 standard passives, got %d", len(passives)))
	}

	catalogPayload, err := os.ReadFile(*catalogPath)
	check(err)
	var raw map[string]json.RawMessage
	check(json.Unmarshal(catalogPayload, &raw))
	for _, key := range []string{"source", "pals", "items", "item_icons"} {
		if len(raw[key]) == 0 {
			check(fmt.Errorf("catalog is missing %q", key))
		}
	}
	out := outputCatalog{
		Source: raw["source"],
		PassiveSource: sourceInfo{
			Project: "tylercamp/palcalc", Version: database.Version, Commit: strings.TrimSpace(*commit), License: "MIT",
		},
		Pals: raw["pals"], Items: raw["items"], Passives: passives, ItemIcons: raw["item_icons"],
	}
	if out.PassiveSource.Commit == "" {
		check(fmt.Errorf("PalCalc commit is empty"))
	}

	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	check(encoder.Encode(out))
	temporary := *catalogPath + ".tmp"
	check(os.WriteFile(temporary, buffer.Bytes(), 0o644))
	check(os.Rename(temporary, *catalogPath))

	ids := make([]string, 0, len(passives))
	for id := range passives {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	fmt.Printf("wrote %d PalCalc %s standard passives to %s\n", len(ids), database.Version, *catalogPath)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
