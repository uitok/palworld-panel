package main

import (
	"log"

	"palpanel/internal/api"
	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
	"palpanel/internal/mods"
	"palpanel/internal/paldefender"
	"palpanel/internal/palrest"
	"palpanel/internal/server"
)

func main() {
	cfg, err := appconfig.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		log.Fatalf("ensure data directories: %v", err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()

	runner := docker.NewRunner(cfg)
	serverManager := server.NewManager(cfg, store, runner)
	modsManager := mods.NewManager(cfg, store, runner)
	palDefenderManager := paldefender.NewManager(cfg, store)
	restClient := palrest.New(cfg.PalworldRESTBaseURL, cfg.PalworldRESTUser, cfg.PalworldRESTPass)

	router := api.NewRouter(cfg, store, serverManager, modsManager, palDefenderManager, restClient)
	log.Printf("palpanel backend listening on %s", cfg.ListenAddr)
	log.Printf("data directory: %s", cfg.DataDir)
	if !cfg.RequireAuth {
		log.Printf("warning: PALPANEL_REQUIRE_AUTH is disabled")
	}
	if err := router.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("run api: %v", err)
	}
}
