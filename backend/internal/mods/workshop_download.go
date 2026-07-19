package mods

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"palpanel/internal/server"
	"palpanel/internal/steamcmd"
)

type nativeWorkshopDownloader interface {
	Ensure(context.Context) error
	DownloadWorkshopTo(context.Context, string, string, string, string) error
}

type workshopAuthenticator interface {
	LoginStatus(string) steamcmd.LoginStatus
	StartInteractiveLogin(context.Context, string) (steamcmd.LoginStatus, error)
	VerifyLogin(context.Context, string) (steamcmd.LoginStatus, error)
	RequireLogin(context.Context, string) (steamcmd.LoginStatus, error)
}

func (m Manager) workshopRuntimeMode(ctx context.Context) (string, error) {
	mode, configured, err := m.store.GetKV(ctx, "runtime_mode")
	if err != nil {
		return "", err
	}
	mode = strings.TrimSpace(mode)
	if !configured || (mode != server.RuntimeWindowsSteamCMD && mode != server.RuntimeWineDocker) {
		return server.RecommendedRuntimeForOS(runtime.GOOS), nil
	}
	return mode, nil
}

func (m Manager) downloadWorkshopTo(ctx context.Context, jobID, itemID, destination string) error {
	mode, err := m.workshopRuntimeMode(ctx)
	if err != nil {
		return fmt.Errorf("read runtime mode: %w", err)
	}
	if mode == server.RuntimeWindowsSteamCMD {
		accountName, _, err := m.store.GetKV(ctx, workshopSteamAccountKey)
		if err != nil {
			return fmt.Errorf("read Steam Workshop account: %w", err)
		}
		m.update(jobID, "running", 10, "preparing native SteamCMD", "")
		if err := m.native.Ensure(ctx); err != nil {
			return fmt.Errorf("prepare native SteamCMD: %w", err)
		}
		m.update(jobID, "running", 50, "downloading Steam Workshop item with native SteamCMD", "")
		if err := m.native.DownloadWorkshopTo(ctx, m.cfg.WorkshopAppID, itemID, destination, accountName); err != nil {
			return fmt.Errorf("native SteamCMD Workshop download: %w", err)
		}
		return nil
	}

	m.update(jobID, "running", 10, "building wine runner image", "")
	if err := m.runner.BuildImage(ctx); err != nil {
		return fmt.Errorf("build wine runner image: %w", err)
	}
	m.update(jobID, "running", 50, "downloading Steam Workshop item", "")
	accountName, _, err := m.store.GetKV(ctx, workshopSteamAccountKey)
	if err != nil {
		return fmt.Errorf("read Steam Workshop account: %w", err)
	}
	if err := m.runner.DownloadWorkshopTo(ctx, itemID, destination, accountName); err != nil {
		return fmt.Errorf("Docker/Wine Workshop download: %w", err)
	}
	return nil
}
