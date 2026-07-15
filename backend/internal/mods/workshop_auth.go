package mods

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"palpanel/internal/server"
	"palpanel/internal/steamcmd"
)

const workshopSteamAccountKey = "steam_workshop_account_name"

var ErrSteamAccountRequired = errors.New("Steam account name is required")

func (m Manager) WorkshopAuthStatus(ctx context.Context) (steamcmd.LoginStatus, error) {
	dockerAuth, err := m.usesDockerWorkshopAuth(ctx)
	if err != nil {
		return steamcmd.LoginStatus{}, err
	}
	if dockerAuth {
		return dockerWorkshopAuthStatus(), nil
	}
	accountName, _, err := m.store.GetKV(ctx, workshopSteamAccountKey)
	if err != nil {
		return steamcmd.LoginStatus{}, fmt.Errorf("read Steam Workshop account: %w", err)
	}
	status := m.steamAuth.LoginStatus(accountName)
	if accountName == "" || !status.Supported || !status.SteamCMDInstalled || status.LoggedIn || status.LoginInProgress {
		return status, nil
	}
	status, err = m.steamAuth.VerifyLogin(ctx, accountName)
	if err != nil {
		return status, fmt.Errorf("verify cached Steam Workshop session: %w", err)
	}
	return status, nil
}

func (m Manager) StartWorkshopLogin(ctx context.Context, accountName string) (steamcmd.LoginStatus, error) {
	dockerAuth, err := m.usesDockerWorkshopAuth(ctx)
	if err != nil {
		return steamcmd.LoginStatus{}, err
	}
	if dockerAuth {
		status := dockerWorkshopAuthStatus()
		return status, fmt.Errorf("%w: Docker/Wine Workshop credentials must be configured with STEAM_USERNAME and STEAM_PASSWORD in palpanel.env, then PalPanel must be restarted", steamcmd.ErrInteractiveLogin)
	}
	accountName, err = m.resolveWorkshopAccount(ctx, accountName)
	if err != nil {
		return steamcmd.LoginStatus{}, err
	}
	if err := steamcmd.ValidateAccountName(accountName); err != nil {
		return m.steamAuth.LoginStatus(accountName), err
	}
	if err := m.store.SetKV(ctx, workshopSteamAccountKey, accountName); err != nil {
		return m.steamAuth.LoginStatus(accountName), fmt.Errorf("save Steam Workshop account: %w", err)
	}
	return m.steamAuth.StartInteractiveLogin(ctx, accountName)
}

func (m Manager) VerifyWorkshopLogin(ctx context.Context, accountName string) (steamcmd.LoginStatus, error) {
	dockerAuth, err := m.usesDockerWorkshopAuth(ctx)
	if err != nil {
		return steamcmd.LoginStatus{}, err
	}
	if dockerAuth {
		status := dockerWorkshopAuthStatus()
		if status.LoggedIn {
			return status, nil
		}
		return status, fmt.Errorf("%w: Docker/Wine Workshop credentials must be configured with STEAM_USERNAME and STEAM_PASSWORD in palpanel.env, then PalPanel must be restarted", steamcmd.ErrInteractiveLogin)
	}
	accountName, err = m.resolveWorkshopAccount(ctx, accountName)
	if err != nil {
		return steamcmd.LoginStatus{}, err
	}
	if err := steamcmd.ValidateAccountName(accountName); err != nil {
		return m.steamAuth.LoginStatus(accountName), err
	}
	if err := m.store.SetKV(ctx, workshopSteamAccountKey, accountName); err != nil {
		return m.steamAuth.LoginStatus(accountName), fmt.Errorf("save Steam Workshop account: %w", err)
	}
	return m.steamAuth.VerifyLogin(ctx, accountName)
}

func (m Manager) RequireWorkshopLogin(ctx context.Context) (steamcmd.LoginStatus, error) {
	dockerAuth, err := m.usesDockerWorkshopAuth(ctx)
	if err != nil {
		return steamcmd.LoginStatus{}, err
	}
	if dockerAuth {
		status := dockerWorkshopAuthStatus()
		if status.LoggedIn {
			return status, nil
		}
		return status, fmt.Errorf("%w: set STEAM_USERNAME and STEAM_PASSWORD in palpanel.env and restart PalPanel before using Workshop", steamcmd.ErrLoginRequired)
	}
	accountName, _, err := m.store.GetKV(ctx, workshopSteamAccountKey)
	if err != nil {
		return steamcmd.LoginStatus{}, fmt.Errorf("read Steam Workshop account: %w", err)
	}
	if strings.TrimSpace(accountName) == "" {
		return m.steamAuth.LoginStatus(""), steamcmd.ErrLoginRequired
	}
	return m.steamAuth.RequireLogin(ctx, accountName)
}

func (m Manager) usesDockerWorkshopAuth(ctx context.Context) (bool, error) {
	mode, err := m.workshopRuntimeMode(ctx)
	if err != nil {
		return false, fmt.Errorf("read runtime mode: %w", err)
	}
	return mode == server.RuntimeWineDocker, nil
}

func dockerWorkshopAuthStatus() steamcmd.LoginStatus {
	accountName := strings.TrimSpace(os.Getenv("STEAM_USERNAME"))
	passwordConfigured := os.Getenv("STEAM_PASSWORD") != ""
	configured := accountName != "" && passwordConfigured
	status := steamcmd.LoginStatus{
		// The Windows-only interactive launcher is intentionally unavailable in
		// Docker/Wine mode. A complete mode-0600 environment configuration is the
		// Linux authentication mechanism instead.
		Supported:            false,
		SteamCMDInstalled:    false,
		CredentialsSecure:    configured,
		LoggedIn:             configured,
		VerificationRequired: !configured,
		AccountName:          accountName,
	}
	if configured {
		status.Message = "Docker/Wine Workshop credentials are configured in palpanel.env; secret values are not returned by the API or placed in Docker command arguments."
	} else {
		status.Message = "Set both STEAM_USERNAME and STEAM_PASSWORD in palpanel.env, keep the file mode 0600, then restart PalPanel. Interactive SteamCMD login is available only on native Windows."
	}
	return status
}

func (m Manager) resolveWorkshopAccount(ctx context.Context, supplied string) (string, error) {
	accountName := strings.TrimSpace(supplied)
	if accountName != "" {
		return accountName, nil
	}
	stored, configured, err := m.store.GetKV(ctx, workshopSteamAccountKey)
	if err != nil {
		return "", fmt.Errorf("read Steam Workshop account: %w", err)
	}
	stored = strings.TrimSpace(stored)
	if !configured || stored == "" {
		return "", ErrSteamAccountRequired
	}
	return stored, nil
}
