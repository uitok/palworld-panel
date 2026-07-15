package mods

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"palpanel/internal/steamcmd"
)

const workshopSteamAccountKey = "steam_workshop_account_name"

var ErrSteamAccountRequired = errors.New("Steam account name is required")

func (m Manager) WorkshopAuthStatus(ctx context.Context) (steamcmd.LoginStatus, error) {
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
	accountName, err := m.resolveWorkshopAccount(ctx, accountName)
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
	accountName, err := m.resolveWorkshopAccount(ctx, accountName)
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
	accountName, _, err := m.store.GetKV(ctx, workshopSteamAccountKey)
	if err != nil {
		return steamcmd.LoginStatus{}, fmt.Errorf("read Steam Workshop account: %w", err)
	}
	if strings.TrimSpace(accountName) == "" {
		return m.steamAuth.LoginStatus(""), steamcmd.ErrLoginRequired
	}
	return m.steamAuth.RequireLogin(ctx, accountName)
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
