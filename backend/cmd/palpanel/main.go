package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"palpanel/internal/api"
	"palpanel/internal/appconfig"
	"palpanel/internal/buildinfo"
	"palpanel/internal/db"
	"palpanel/internal/docker"
	"palpanel/internal/mods"
	"palpanel/internal/monitor"
	"palpanel/internal/paldefender"
	"palpanel/internal/palrest"
	"palpanel/internal/scheduler"
	"palpanel/internal/server"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("palpanel: %v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("palpanel", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to palpanel.env")
	initConfig := fs.Bool("init-config", false, "create a secure production configuration")
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if *showVersion {
		info := buildinfo.Current()
		fmt.Printf("palpanel %s (commit %s, built %s)\n", info.Version, info.Commit, info.BuildTime)
		return nil
	}
	if *initConfig {
		path := *configPath
		if path == "" {
			path = "palpanel.env"
		}
		token, created, err := appconfig.InitFile(path)
		if err != nil {
			return fmt.Errorf("initialize config: %w", err)
		}
		if created {
			absolute, _ := filepath.Abs(path)
			fmt.Printf("created %s\n", absolute)
			fmt.Printf("PANEL_TOKEN=%s\n", token)
		} else {
			fmt.Printf("config already exists: %s\n", path)
		}
		return nil
	}

	if *configPath != "" {
		values, err := appconfig.ParseEnvFile(*configPath)
		if err != nil {
			return fmt.Errorf("load config file: %w", err)
		}
		if _, err := appconfig.ApplyFileEnvironment(values); err != nil {
			return fmt.Errorf("load config file: %w", err)
		}
	}
	cfg, err := appconfig.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		return fmt.Errorf("ensure data directories: %w", err)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	runner := docker.NewRunner(cfg)
	serverManager := server.NewManager(cfg, store, runner)
	modsManager := mods.NewManager(cfg, store, runner)
	palDefenderManager := paldefender.NewManager(cfg, store)
	restClient := palrest.New(cfg.PalworldRESTBaseURL, cfg.PalworldRESTUser, cfg.PalworldRESTPass)
	monitorManager := monitor.New(cfg, store, serverManager, restClient)
	schedulerManager := scheduler.New(store, serverManager, restClient)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	monitorDone := monitorManager.Start(ctx)
	schedulerDone := schedulerManager.Start(ctx)
	defer func() {
		stop()
		workerCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = waitForBackground(workerCtx, monitorDone, schedulerDone)
	}()

	router := api.NewRouter(cfg, store, serverManager, modsManager, palDefenderManager, restClient, monitorManager, schedulerManager)
	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	info := buildinfo.Current()
	log.Printf("palpanel %s listening on %s", info.Version, cfg.ListenAddr)
	log.Printf("data directory: %s", cfg.DataDir)
	if !cfg.RequireAuth {
		log.Printf("warning: PALPANEL_REQUIRE_AUTH is disabled")
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("run api: %w", err)
		}
		return nil
	case <-ctx.Done():
		log.Printf("shutdown requested")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := waitForBackground(shutdownCtx, monitorDone, schedulerDone); err != nil {
		return err
	}
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("run api: %w", err)
	}
	log.Printf("shutdown complete")
	return nil
}

func waitForBackground(ctx context.Context, workers ...<-chan struct{}) error {
	for _, worker := range workers {
		select {
		case <-worker:
		case <-ctx.Done():
			return fmt.Errorf("wait for background workers: %w", ctx.Err())
		}
	}
	return nil
}
