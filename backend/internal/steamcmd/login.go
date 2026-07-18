package steamcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	loginVerificationTimeout = 45 * time.Second
	loginCacheTTL            = 15 * time.Minute
)

var (
	ErrLoginRequired      = errors.New("Steam login is required; open the SteamCMD login window and verify the cached session before downloading Workshop Mods")
	ErrLoginInProgress    = errors.New("SteamCMD login window is still open; complete login, enter quit, and wait for the window to close")
	ErrInteractiveLogin   = errors.New("interactive SteamCMD login is supported only on a Windows host")
	ErrInvalidAccountName = errors.New("invalid Steam account name")
	steamAccountNameRegex = regexp.MustCompile(`^[A-Za-z0-9_]{3,64}$`)
)

type credentialHardener func(context.Context, string) error
type interactiveLauncher func(string, string, string) (<-chan struct{}, error)
type sessionVerifier func(context.Context, string) (bool, error)

// LoginStatus contains only non-secret account and verification metadata.
// Passwords, Steam Guard codes, refresh tokens, and SteamCMD config contents
// never enter this type.
type LoginStatus struct {
	Supported            bool   `json:"supported"`
	SteamCMDInstalled    bool   `json:"steamcmd_installed"`
	CredentialsSecure    bool   `json:"credentials_secure"`
	LoginInProgress      bool   `json:"login_in_progress"`
	LoggedIn             bool   `json:"logged_in"`
	VerificationRequired bool   `json:"verification_required"`
	AccountName          string `json:"account_name,omitempty"`
	LastVerifiedAt       string `json:"last_verified_at,omitempty"`
	Message              string `json:"message,omitempty"`
}

type loginState struct {
	account           string
	verifiedAt        time.Time
	loginInProgress   bool
	credentialsSecure bool
	attempt           uint64
}

func ValidateAccountName(accountName string) error {
	accountName = strings.TrimSpace(accountName)
	if accountName == "" {
		return fmt.Errorf("%w: account name is required", ErrInvalidAccountName)
	}
	if !steamAccountNameRegex.MatchString(accountName) {
		return fmt.Errorf("%w: account name must contain 3-64 ASCII letters, digits, or underscores", ErrInvalidAccountName)
	}
	return nil
}

func (c *Client) LoginStatus(accountName string) LoginStatus {
	accountName = strings.TrimSpace(accountName)
	status := LoginStatus{
		Supported:         c.goos == "windows",
		SteamCMDInstalled: c.validateInstalled() == nil,
		AccountName:       accountName,
	}
	if !status.Supported {
		status.VerificationRequired = true
		status.Message = ErrInteractiveLogin.Error()
		return status
	}
	if accountName == "" {
		status.VerificationRequired = true
		status.Message = "Enter the Steam account name used for the local SteamCMD session."
		return status
	}

	c.loginMu.Lock()
	defer c.loginMu.Unlock()
	status.CredentialsSecure = c.login.credentialsSecure
	if c.login.account == accountName {
		status.LoginInProgress = c.login.loginInProgress
	}
	if c.login.account == accountName && !c.login.verifiedAt.IsZero() {
		status.LastVerifiedAt = c.login.verifiedAt.UTC().Format(time.RFC3339)
		elapsed := c.now().Sub(c.login.verifiedAt)
		status.LoggedIn = elapsed >= 0 && elapsed <= loginCacheTTL
	}
	status.VerificationRequired = !status.LoggedIn
	if status.LoginInProgress {
		status.Message = "Complete login in the SteamCMD window, enter quit, then verify the session."
	} else if status.VerificationRequired {
		status.Message = "Verify the cached SteamCMD session before downloading Workshop Mods."
	}
	return status
}

func (c *Client) StartInteractiveLogin(ctx context.Context, accountName string) (LoginStatus, error) {
	accountName = strings.TrimSpace(accountName)
	if c.goos != "windows" {
		return c.LoginStatus(accountName), ErrInteractiveLogin
	}
	if err := ValidateAccountName(accountName); err != nil {
		return c.LoginStatus(accountName), err
	}
	if status := c.LoginStatus(accountName); status.LoginInProgress {
		return status, ErrLoginInProgress
	}
	if err := c.Ensure(ctx); err != nil {
		return c.LoginStatus(accountName), err
	}
	release, err := c.acquire(ctx)
	if err != nil {
		return c.LoginStatus(accountName), err
	}
	releaseOnReturn := true
	defer func() {
		if releaseOnReturn {
			release()
		}
	}()
	if err := c.validateInstalled(); err != nil {
		return c.LoginStatus(accountName), err
	}
	if err := c.hardenCredentials(ctx); err != nil {
		return c.LoginStatus(accountName), err
	}
	if c.interactiveLauncher == nil {
		return c.LoginStatus(accountName), fmt.Errorf("SteamCMD interactive launcher is unavailable")
	}
	done, err := c.interactiveLauncher(c.cfg.SteamCMDBinaryPath(), c.cfg.SteamCMDDir, accountName)
	if err != nil {
		return c.LoginStatus(accountName), fmt.Errorf("open SteamCMD login window: %w", err)
	}
	if done == nil {
		return c.LoginStatus(accountName), errors.New("SteamCMD interactive launcher did not return process status")
	}
	c.loginMu.Lock()
	c.login.attempt++
	attempt := c.login.attempt
	c.login.account = accountName
	c.login.verifiedAt = time.Time{}
	c.login.loginInProgress = true
	c.login.credentialsSecure = true
	c.loginMu.Unlock()
	releaseOnReturn = false
	go c.watchInteractiveLogin(accountName, attempt, done, release)
	return c.LoginStatus(accountName), nil
}

func (c *Client) watchInteractiveLogin(accountName string, attempt uint64, done <-chan struct{}, release func()) {
	defer release()
	<-done
	c.loginMu.Lock()
	if c.login.account == accountName && c.login.attempt == attempt {
		c.login.loginInProgress = false
	}
	c.loginMu.Unlock()
}

func (c *Client) VerifyLogin(ctx context.Context, accountName string) (LoginStatus, error) {
	accountName = strings.TrimSpace(accountName)
	if c.goos != "windows" {
		return c.LoginStatus(accountName), ErrInteractiveLogin
	}
	if err := ValidateAccountName(accountName); err != nil {
		return c.LoginStatus(accountName), err
	}
	if status := c.LoginStatus(accountName); status.LoginInProgress {
		return status, ErrLoginInProgress
	}
	if err := c.Ensure(ctx); err != nil {
		return c.LoginStatus(accountName), err
	}
	if err := c.hardenCredentials(ctx); err != nil {
		return c.LoginStatus(accountName), err
	}
	if c.sessionVerifier == nil {
		return c.LoginStatus(accountName), errors.New("SteamCMD cached-session verifier is unavailable")
	}
	verifyCtx, cancel := context.WithTimeout(ctx, loginVerificationTimeout)
	defer cancel()
	verified, err := c.sessionVerifier(verifyCtx, accountName)

	c.loginMu.Lock()
	c.login.account = accountName
	c.login.loginInProgress = false
	c.login.credentialsSecure = true
	if verified && err == nil {
		c.login.verifiedAt = c.now()
	} else {
		c.login.verifiedAt = time.Time{}
	}
	c.loginMu.Unlock()
	status := c.LoginStatus(accountName)
	if err != nil {
		status.Message = "SteamCMD cached-session verification could not be completed."
		return status, err
	}
	if !verified {
		status.Message = "Cached SteamCMD credentials are missing or expired. Open the login window and sign in again."
	}
	return status, nil
}

func (c *Client) RequireLogin(ctx context.Context, accountName string) (LoginStatus, error) {
	if strings.TrimSpace(accountName) == "" {
		return c.LoginStatus(accountName), ErrLoginRequired
	}
	status := c.LoginStatus(accountName)
	if status.LoggedIn && status.CredentialsSecure {
		return status, nil
	}
	status, err := c.VerifyLogin(ctx, accountName)
	if err != nil {
		return status, err
	}
	if !status.LoggedIn {
		return status, ErrLoginRequired
	}
	return status, nil
}

func (c *Client) hardenCredentials(ctx context.Context) error {
	if c.credentialHardener == nil {
		return fmt.Errorf("SteamCMD credential ACL hardener is unavailable")
	}
	if err := c.validateManaged(c.cfg.SteamCMDDir); err != nil {
		return err
	}
	configDir := filepath.Join(c.cfg.SteamCMDDir, "config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("create SteamCMD config directory: %w", err)
	}
	if err := c.validateManaged(configDir); err != nil {
		return fmt.Errorf("validate SteamCMD credential directory: %w", err)
	}
	if err := c.credentialHardener(ctx, c.cfg.SteamCMDDir); err != nil {
		return fmt.Errorf("secure SteamCMD credential files: %w", err)
	}
	if err := c.validateManaged(configDir); err != nil {
		return fmt.Errorf("revalidate SteamCMD credential directory: %w", err)
	}
	c.loginMu.Lock()
	c.login.credentialsSecure = true
	c.loginMu.Unlock()
	return nil
}

func (c *Client) verifyCachedSession(ctx context.Context, accountName string) (bool, error) {
	out, err := c.executeRedacted(ctx, []string{accountName},
		"+@ShutdownOnFailedCommand", "1",
		"+@NoPromptForPassword", "1",
		"+login", accountName,
		"+quit",
	)
	if err != nil {
		if loginFailureOutput(out) {
			return false, nil
		}
		return false, err
	}
	return loginSuccessOutput(out) && !loginFailureOutput(out), nil
}

func loginSuccessOutput(output []byte) bool {
	lower := strings.ToLower(string(output))
	return strings.Contains(lower, "waiting for user info...ok") ||
		strings.Contains(lower, "logged in ok") ||
		strings.Contains(lower, "login successful") ||
		strings.Contains(lower, "logging in using cached credentials") && strings.Contains(lower, "steam public...ok")
}

func loginFailureOutput(output []byte) bool {
	lower := strings.ToLower(string(output))
	markers := []string{
		"invalid password",
		"account logon denied",
		"steam guard",
		"two-factor",
		"no cached credentials",
		"password required",
		"login failure",
		"failed to log in",
		"not logged on",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func (c *Client) invalidateLogin() {
	c.loginMu.Lock()
	c.login.verifiedAt = time.Time{}
	c.login.loginInProgress = false
	c.loginMu.Unlock()
}
