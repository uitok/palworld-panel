package mods

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
		accountName, _, err := m.store.GetKV(ctx, workshopSteamAccountKey)
		if err != nil {
			return steamcmd.LoginStatus{}, fmt.Errorf("read Steam Workshop account: %w", err)
		}
		return m.dockerWorkshopAuthStatus(ctx, accountName, strings.TrimSpace(accountName) != "")
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
		accountName, err = m.resolveWorkshopAccount(ctx, accountName)
		if err != nil {
			return steamcmd.LoginStatus{}, err
		}
		if err := steamcmd.ValidateAccountName(accountName); err != nil {
			return steamcmd.LoginStatus{Supported: true, AccountName: accountName, VerificationRequired: true}, err
		}
		if err := m.store.SetKV(ctx, workshopSteamAccountKey, accountName); err != nil {
			return steamcmd.LoginStatus{}, fmt.Errorf("save Steam Workshop account: %w", err)
		}
		status, statusErr := m.dockerWorkshopAuthStatus(ctx, accountName, false)
		if statusErr != nil {
			return status, statusErr
		}
		return status, fmt.Errorf("%w: on the Linux server run `palpanelctl steam-login %s`, complete password and Steam Guard prompts in SteamCMD, enter quit, then return here and verify", steamcmd.ErrInteractiveLogin, accountName)
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
		accountName, err = m.resolveWorkshopAccount(ctx, accountName)
		if err != nil {
			return steamcmd.LoginStatus{}, err
		}
		if err := steamcmd.ValidateAccountName(accountName); err != nil {
			return steamcmd.LoginStatus{Supported: true, AccountName: accountName, VerificationRequired: true}, err
		}
		if err := m.store.SetKV(ctx, workshopSteamAccountKey, accountName); err != nil {
			return steamcmd.LoginStatus{}, fmt.Errorf("save Steam Workshop account: %w", err)
		}
		return m.dockerWorkshopAuthStatus(ctx, accountName, true)
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
		accountName, _, err := m.store.GetKV(ctx, workshopSteamAccountKey)
		if err != nil {
			return steamcmd.LoginStatus{}, fmt.Errorf("read Steam Workshop account: %w", err)
		}
		if strings.TrimSpace(accountName) == "" {
			return steamcmd.LoginStatus{Supported: true, VerificationRequired: true}, steamcmd.ErrLoginRequired
		}
		status, err := m.dockerWorkshopAuthStatus(ctx, accountName, true)
		if err != nil {
			return status, err
		}
		if !status.LoggedIn {
			return status, fmt.Errorf("%w: run `palpanelctl steam-login %s` on the Linux server, complete SteamCMD login, then verify again", steamcmd.ErrLoginRequired, accountName)
		}
		return status, nil
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

func (m Manager) dockerWorkshopAuthStatus(ctx context.Context, accountName string, verify bool) (steamcmd.LoginStatus, error) {
	accountName = strings.TrimSpace(accountName)
	imageExists, err := m.runner.ImageExists(ctx)
	if err != nil {
		return steamcmd.LoginStatus{}, fmt.Errorf("check Docker/Wine SteamCMD runner: %w", err)
	}
	status := steamcmd.LoginStatus{
		Supported:            true,
		SteamCMDInstalled:    imageExists,
		CredentialsSecure:    m.runner.WorkshopCredentialsSecure(),
		VerificationRequired: true,
		AccountName:          accountName,
	}
	if !imageExists {
		status.Message = "The Docker/Wine SteamCMD runner image is not built yet. Install or update the server once, then run palpanelctl steam-login on the Linux server."
		return status, nil
	}
	if accountName == "" {
		status.Message = "Enter a Steam account name. On Linux, PalPanel stores only SteamCMD's verified login cache; passwords and Steam Guard codes stay in the local SteamCMD terminal."
		return status, nil
	}
	if !verify {
		status.Message = fmt.Sprintf("Run `palpanelctl steam-login %s` on the Linux server, complete SteamCMD login, enter quit, then verify the cached session here.", accountName)
		return status, nil
	}
	verified, err := m.runner.VerifyWorkshopLogin(ctx, accountName)
	status.CredentialsSecure = m.runner.WorkshopCredentialsSecure()
	if err != nil {
		status.Message = "SteamCMD cached-session verification could not be completed."
		return status, fmt.Errorf("verify Docker/Wine SteamCMD cache: %w", err)
	}
	status.LoggedIn = verified && status.CredentialsSecure
	status.VerificationRequired = !status.LoggedIn
	if status.LoggedIn {
		status.LastVerifiedAt = time.Now().UTC().Format(time.RFC3339)
		status.Message = "SteamCMD cached credentials were verified successfully. Workshop downloads will re-check this session before use."
	} else {
		status.Message = fmt.Sprintf("Cached SteamCMD credentials are missing or expired. Run `palpanelctl steam-login %s` on the Linux server and sign in again.", accountName)
	}
	return status, nil
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
