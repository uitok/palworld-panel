package mods

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"palpanel/internal/server"
	"palpanel/internal/steamcmd"
)

const workshopSteamAccountKey = "steam_workshop_account_name"

var ErrSteamAccountRequired = errors.New("Steam account name is required")

func (m Manager) WorkshopAuthStatus(ctx context.Context) (steamcmd.LoginStatus, error) {
	accountName, _, err := m.store.GetKV(ctx, workshopSteamAccountKey)
	if err != nil {
		return steamcmd.LoginStatus{}, fmt.Errorf("read Steam Workshop account: %w", err)
	}
	status, err := m.steamAuth.CredentialStatus(accountName)
	if err != nil {
		return status, fmt.Errorf("read Steam Workshop credentials: %w", err)
	}
	mode, modeErr := m.workshopRuntimeMode(ctx)
	if modeErr != nil {
		return status, fmt.Errorf("read runtime mode: %w", modeErr)
	}
	if mode != server.RuntimeWindowsSteamCMD {
		installed, imageErr := m.runner.ImageExists(ctx)
		if imageErr != nil {
			return status, fmt.Errorf("check Docker/Wine SteamCMD runner: %w", imageErr)
		}
		status.SteamCMDInstalled = installed
		if !installed {
			status.LoggedIn = false
			status.VerificationRequired = true
			status.Message = "Install or update the server before configuring Workshop credentials."
		}
	}
	return status, nil
}

func (m Manager) StartWorkshopLogin(ctx context.Context, request steamcmd.LoginRequest) (steamcmd.LoginStatus, error) {
	status, err := m.steamAuth.Authenticate(ctx, request)
	if err != nil {
		return status, err
	}
	if err := m.store.SetKV(ctx, workshopSteamAccountKey, status.AccountName); err != nil {
		return status, fmt.Errorf("save Steam Workshop account: %w", err)
	}
	return status, nil
}

func (m Manager) VerifyWorkshopLogin(ctx context.Context, accountName, steamGuardCode string) (steamcmd.LoginStatus, error) {
	if strings.TrimSpace(accountName) == "" {
		status, statusErr := m.steamAuth.CredentialStatus("")
		if statusErr != nil {
			return status, statusErr
		}
		accountName = status.AccountName
		if strings.TrimSpace(accountName) == "" {
			accountName, statusErr = m.resolveWorkshopAccount(ctx, "")
			if statusErr != nil {
				return status, statusErr
			}
		}
	}
	status, err := m.steamAuth.VerifyCredentials(ctx, accountName, steamGuardCode)
	if err != nil {
		return status, err
	}
	if err := m.store.SetKV(ctx, workshopSteamAccountKey, status.AccountName); err != nil {
		return status, fmt.Errorf("save Steam Workshop account: %w", err)
	}
	return status, nil
}

func (m Manager) ClearWorkshopLogin(ctx context.Context) (steamcmd.LoginStatus, error) {
	if err := m.store.SetKV(ctx, workshopSteamAccountKey, ""); err != nil {
		return steamcmd.LoginStatus{}, fmt.Errorf("clear Steam Workshop account: %w", err)
	}
	return m.steamAuth.ClearCredentials(ctx)
}

func (m Manager) RequireWorkshopLogin(ctx context.Context) (steamcmd.LoginStatus, error) {
	accountName, _, err := m.store.GetKV(ctx, workshopSteamAccountKey)
	if err != nil {
		return steamcmd.LoginStatus{}, fmt.Errorf("read Steam Workshop account: %w", err)
	}
	return m.steamAuth.RequireCredentials(ctx, accountName)
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
