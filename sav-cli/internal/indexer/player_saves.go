package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"palpanel/sav-cli/internal/gvas"
	"palpanel/sav-cli/internal/sav"
)

var playerInventoryContainerFields = []string{
	"CommonContainerId",
	"DropSlotContainerId",
	"EssentialContainerId",
	"FoodEquipContainerId",
	"PlayerEquipArmorContainerId",
	"WeaponLoadOutContainerId",
}

func normalizePlayerSaves(index *Index, playersDir string) {
	if len(index.Players) == 0 {
		return
	}

	files, err := playerSaveFiles(playersDir)
	if err != nil {
		index.Warnings = append(index.Warnings, "list player saves: "+err.Error())
		return
	}
	containers := make(map[string]int, len(index.Containers))
	for position := range index.Containers {
		containers[canonicalSaveID(index.Containers[position].ContainerID)] = position
	}

	type result struct {
		containerIDs []string
		err          error
	}
	parsed := map[string]result{}
	for playerPosition := range index.Players {
		player := &index.Players[playerPosition]
		filename := playerSaveFilename(player.PlayerUID)
		cacheKey := strings.ToLower(filename)
		inventory, found := parsed[cacheKey]
		if !found {
			path, exists := files[cacheKey]
			if !exists {
				inventory.err = fmt.Errorf("%s is missing", filename)
			} else {
				file, err := readPlayerSave(path)
				if err != nil {
					inventory.err = fmt.Errorf("%s could not be parsed: %w", filepath.Base(path), err)
				} else {
					inventory.containerIDs, inventory.err = playerInventoryContainerIDs(file)
					if inventory.err != nil {
						inventory.err = fmt.Errorf("%s has invalid inventory data: %w", filepath.Base(path), inventory.err)
					}
				}
			}
			parsed[cacheKey] = inventory
			if inventory.err != nil {
				index.Warnings = append(index.Warnings, "player save warning: "+inventory.err.Error())
			}
		}
		if inventory.err != nil {
			continue
		}
		for _, containerID := range inventory.containerIDs {
			if position, ok := containers[canonicalSaveID(containerID)]; ok {
				index.Containers[position].OwnerType = "player"
				index.Containers[position].OwnerID = player.PlayerUID
			}
		}
	}
}

func playerSaveFiles(playersDir string) (map[string]string, error) {
	entries, err := os.ReadDir(playersDir)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".sav") {
			continue
		}
		files[strings.ToLower(entry.Name())] = filepath.Join(playersDir, entry.Name())
	}
	return files, nil
}

func readPlayerSave(path string) (*gvas.File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	gvasData, _, err := sav.DecodeToGVAS(data)
	if err != nil {
		return nil, fmt.Errorf("decode save: %w", err)
	}
	file, err := gvas.Read(gvasData)
	if err != nil {
		return nil, fmt.Errorf("parse GVAS: %w", err)
	}
	return file, nil
}

func playerInventoryContainerIDs(file *gvas.File) ([]string, error) {
	saveData, ok := asMap(getField(file.Properties, "SaveData"))
	if !ok {
		return nil, fmt.Errorf("SaveData is missing or has an unexpected shape")
	}
	inventoryValue := getField(saveData, "InventoryInfo")
	if inventoryValue == nil {
		return []string{}, nil
	}
	inventory, ok := asMap(inventoryValue)
	if !ok {
		return nil, fmt.Errorf("InventoryInfo has an unexpected shape")
	}

	unique := map[string]string{}
	for _, field := range playerInventoryContainerFields {
		containerID := containerIDFromAny(getField(inventory, field))
		canonical := canonicalSaveID(containerID)
		if canonical == "" || isZeroGUID(canonical) {
			continue
		}
		unique[canonical] = containerID
	}
	containerIDs := make([]string, 0, len(unique))
	for _, containerID := range unique {
		containerIDs = append(containerIDs, containerID)
	}
	sort.Strings(containerIDs)
	return containerIDs, nil
}

func playerSaveFilename(playerUID string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(playerUID), "-", "")) + ".sav"
}

func canonicalSaveID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
