package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"palpanel/internal/db"
	"palpanel/internal/id"
	"palpanel/internal/saveindex"
)

func breedingRequestSubject(c *gin.Context) string {
	if principal := currentBreedPrincipal(c); principal.Subject != "" {
		return principal.Subject
	}
	return breedingSubject(CurrentPrincipal(c))
}

func (s Server) listBreedingPresets(c *gin.Context) {
	items, err := s.store.ListBreedingPresets(c.Request.Context(), breedingRequestSubject(c))
	if err != nil {
		fail(c, http.StatusInternalServerError, "breeding_presets_failed", err.Error())
		return
	}
	response := make([]gin.H, 0, len(items))
	for _, item := range items {
		var config any
		_ = json.Unmarshal([]byte(item.ConfigJSON), &config)
		response = append(response, gin.H{"id": item.ID, "name": item.Name, "config": config, "created_at": item.CreatedAt, "updated_at": item.UpdatedAt})
	}
	ok(c, response)
}

func (s Server) putBreedingPreset(c *gin.Context) {
	var input struct {
		ID     string          `json:"id"`
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
	}
	if c.ShouldBindJSON(&input) != nil || strings.TrimSpace(input.Name) == "" || len(input.Config) == 0 || !json.Valid(input.Config) {
		fail(c, http.StatusBadRequest, "breeding_preset_invalid", "name and valid config are required")
		return
	}
	if input.ID == "" {
		input.ID = id.New("preset")
	}
	item := db.BreedingPreset{ID: input.ID, Subject: breedingRequestSubject(c), Name: strings.TrimSpace(input.Name), ConfigJSON: string(input.Config)}
	if err := s.store.UpsertBreedingPreset(c.Request.Context(), item); err != nil {
		fail(c, http.StatusConflict, "breeding_preset_save_failed", err.Error())
		return
	}
	ok(c, gin.H{"id": item.ID, "name": item.Name, "config": json.RawMessage(item.ConfigJSON)})
}

func (s Server) deleteBreedingPreset(c *gin.Context) {
	if err := s.store.DeleteBreedingPreset(c.Request.Context(), breedingRequestSubject(c), c.Param("id")); err != nil {
		fail(c, http.StatusNotFound, "breeding_preset_not_found", "preset was not found")
		return
	}
	ok(c, gin.H{"deleted": true})
}

func (s Server) listCustomPalContainers(c *gin.Context) {
	items, err := s.store.ListCustomPalContainers(c.Request.Context(), breedingRequestSubject(c))
	if err != nil {
		fail(c, http.StatusInternalServerError, "custom_containers_failed", err.Error())
		return
	}
	response := make([]gin.H, 0, len(items))
	for _, item := range items {
		var pals any
		_ = json.Unmarshal([]byte(item.PalsJSON), &pals)
		response = append(response, gin.H{"id": item.ID, "name": item.Name, "pals": pals, "created_at": item.CreatedAt, "updated_at": item.UpdatedAt})
	}
	ok(c, response)
}

func (s Server) putCustomPalContainer(c *gin.Context) {
	var input struct {
		ID   string          `json:"id"`
		Name string          `json:"name"`
		Pals json.RawMessage `json:"pals"`
	}
	if c.ShouldBindJSON(&input) != nil || strings.TrimSpace(input.Name) == "" || len(input.Pals) == 0 || !json.Valid(input.Pals) {
		fail(c, http.StatusBadRequest, "custom_container_invalid", "name and valid pals are required")
		return
	}
	var pals []saveindex.Pal
	if json.Unmarshal(input.Pals, &pals) != nil || len(pals) > 500 {
		fail(c, http.StatusBadRequest, "custom_container_invalid", "pals must be an array with at most 500 entries")
		return
	}
	for _, pal := range pals {
		if strings.TrimSpace(pal.CharacterID) == "" {
			fail(c, http.StatusBadRequest, "custom_pal_invalid", "each custom pal requires character_id")
			return
		}
	}
	if input.ID == "" {
		input.ID = id.New("container")
	}
	item := db.CustomPalContainer{ID: input.ID, Subject: breedingRequestSubject(c), Name: strings.TrimSpace(input.Name), PalsJSON: string(input.Pals)}
	if err := s.store.UpsertCustomPalContainer(c.Request.Context(), item); err != nil {
		fail(c, http.StatusConflict, "custom_container_save_failed", err.Error())
		return
	}
	ok(c, gin.H{"id": item.ID, "name": item.Name, "pals": pals})
}

func (s Server) deleteCustomPalContainer(c *gin.Context) {
	if err := s.store.DeleteCustomPalContainer(c.Request.Context(), breedingRequestSubject(c), c.Param("id")); err != nil {
		fail(c, http.StatusNotFound, "custom_container_not_found", "custom container was not found")
		return
	}
	ok(c, gin.H{"deleted": true})
}
