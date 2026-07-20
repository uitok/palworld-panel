package steamcmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	steamCredentialVersion  = 1
	maximumCredentialBytes  = 16 << 10
	maximumSteamPasswordLen = 512
	maximumSteamGuardLen    = 64
)

type storedCredentials struct {
	Version        int    `json:"version"`
	AccountName    string `json:"account_name"`
	Password       string `json:"password"`
	LastVerifiedAt string `json:"last_verified_at,omitempty"`
}

// CredentialStatus reports whether managed Workshop credentials are available.
// It does not contact Steam; Workshop downloads validate the locally approved
// SteamCMD cached session and request reauthorization when that cache expires.
func (c *Client) CredentialStatus(accountName string) (LoginStatus, error) {
	accountName = strings.TrimSpace(accountName)
	supported := c.goos == "windows" || c.loginRunner != nil
	status := LoginStatus{
		Supported:         supported,
		SteamCMDInstalled: supported && (c.goos != "windows" || c.validateInstalled() == nil),
		LoginInProgress:   false,
		AccountName:       accountName,
	}
	if !supported {
		status.VerificationRequired = true
		status.Message = ErrInteractiveLogin.Error()
		return status, nil
	}

	credentials, configured, err := c.readCredentials()
	if err != nil {
		return status, err
	}
	if configured {
		status.AccountName = credentials.AccountName
		status.PasswordConfigured = credentials.Password != ""
		status.CredentialsSecure = status.PasswordConfigured
		status.LastVerifiedAt = credentials.LastVerifiedAt
		status.LoggedIn = status.PasswordConfigured && credentials.LastVerifiedAt != ""
	}
	c.loginMu.Lock()
	status.SteamGuardRequired = c.login.steamGuardRequired
	c.loginMu.Unlock()
	if status.SteamGuardRequired {
		status.LoggedIn = false
	}
	status.VerificationRequired = !status.LoggedIn || status.SteamGuardRequired
	switch {
	case !status.SteamCMDInstalled:
		status.Message = "Install SteamCMD before configuring Workshop credentials."
	case !status.PasswordConfigured:
		status.Message = "Enter the Steam account name and password used for Workshop downloads."
	case status.SteamGuardRequired:
		status.Message = ErrSteamGuardRequired.Error()
	case status.LoggedIn:
		status.Message = "Steam credentials are configured and the local SteamCMD session is authorized for Workshop downloads."
	default:
		status.Message = "Verify the saved Steam credentials before downloading Workshop Mods."
	}
	return status, nil
}

// Authenticate performs an explicit SteamCMD login and persists the account
// name and password only after that login succeeds. Steam Guard codes are used
// for this attempt only and never enter the stored credential structure.
func (c *Client) Authenticate(ctx context.Context, request LoginRequest) (LoginStatus, error) {
	c.credentialMu.Lock()
	defer c.credentialMu.Unlock()
	request.AccountName = strings.TrimSpace(request.AccountName)
	request.SteamGuardCode = strings.TrimSpace(request.SteamGuardCode)
	if c.goos != "windows" && c.loginRunner == nil {
		status, _ := c.CredentialStatus(request.AccountName)
		return status, ErrInteractiveLogin
	}
	if err := validateLoginRequest(request, true); err != nil {
		status, _ := c.CredentialStatus(request.AccountName)
		return status, err
	}
	if err := c.ensureCredentialRuntime(ctx); err != nil {
		status, _ := c.CredentialStatus(request.AccountName)
		return status, err
	}
	if err := c.executeExplicitLogin(ctx, request); err != nil {
		c.setSteamGuardRequired(errors.Is(err, ErrSteamGuardRequired))
		status, _ := c.CredentialStatus(request.AccountName)
		return status, err
	}
	credentials := storedCredentials{
		Version:        steamCredentialVersion,
		AccountName:    request.AccountName,
		Password:       request.Password,
		LastVerifiedAt: c.now().UTC().Format(time.RFC3339),
	}
	if err := c.writeCredentials(ctx, credentials); err != nil {
		status, _ := c.CredentialStatus(request.AccountName)
		return status, err
	}
	c.setSteamGuardRequired(false)
	return c.CredentialStatus(request.AccountName)
}

// VerifyCredentials explicitly logs in with the saved password. A fresh Steam
// Guard code may be supplied when Steam has invalidated the machine grant.
func (c *Client) VerifyCredentials(ctx context.Context, accountName, steamGuardCode string) (LoginStatus, error) {
	c.credentialMu.Lock()
	defer c.credentialMu.Unlock()
	credentials, configured, err := c.readCredentials()
	if err != nil {
		return LoginStatus{}, err
	}
	if !configured || credentials.Password == "" {
		status, _ := c.CredentialStatus(accountName)
		return status, ErrLoginRequired
	}
	accountName = strings.TrimSpace(accountName)
	if accountName != "" && accountName != credentials.AccountName {
		status, _ := c.CredentialStatus(accountName)
		return status, ErrInvalidCredentials
	}
	request := LoginRequest{
		AccountName:    credentials.AccountName,
		Password:       credentials.Password,
		SteamGuardCode: strings.TrimSpace(steamGuardCode),
	}
	if err := validateLoginRequest(request, true); err != nil {
		status, _ := c.CredentialStatus(credentials.AccountName)
		return status, err
	}
	if err := c.ensureCredentialRuntime(ctx); err != nil {
		status, _ := c.CredentialStatus(credentials.AccountName)
		return status, err
	}
	if err := c.executeExplicitLogin(ctx, request); err != nil {
		c.setSteamGuardRequired(errors.Is(err, ErrSteamGuardRequired))
		_ = c.markCredentialsUnverified(ctx)
		status, _ := c.CredentialStatus(credentials.AccountName)
		return status, err
	}
	credentials.LastVerifiedAt = c.now().UTC().Format(time.RFC3339)
	if err := c.writeCredentials(ctx, credentials); err != nil {
		status, _ := c.CredentialStatus(credentials.AccountName)
		return status, err
	}
	c.setSteamGuardRequired(false)
	return c.CredentialStatus(credentials.AccountName)
}

func (c *Client) ClearCredentials(ctx context.Context) (LoginStatus, error) {
	c.credentialMu.Lock()
	defer c.credentialMu.Unlock()
	path := c.cfg.SteamWorkshopCredentialsPath()
	if err := c.validateManaged(path); err != nil {
		return LoginStatus{}, err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return LoginStatus{}, fmt.Errorf("remove Steam Workshop credentials: %w", err)
	}
	c.setSteamGuardRequired(false)
	return c.CredentialStatus("")
}

func (c *Client) RequireCredentials(_ context.Context, accountName string) (LoginStatus, error) {
	status, err := c.CredentialStatus(accountName)
	if err != nil {
		return status, err
	}
	if !status.PasswordConfigured {
		return status, ErrLoginRequired
	}
	return status, nil
}

func (c *Client) explicitCredentials() (storedCredentials, error) {
	credentials, configured, err := c.readCredentials()
	if err != nil {
		return storedCredentials{}, err
	}
	if !configured || credentials.Password == "" {
		return storedCredentials{}, ErrLoginRequired
	}
	return credentials, nil
}

func (c *Client) markCredentialsUnverified(ctx context.Context) error {
	credentials, configured, err := c.readCredentials()
	if err != nil || !configured || credentials.LastVerifiedAt == "" {
		return err
	}
	credentials.LastVerifiedAt = ""
	return c.writeCredentials(ctx, credentials)
}

func validateLoginRequest(request LoginRequest, passwordRequired bool) error {
	if err := ValidateAccountName(request.AccountName); err != nil {
		return err
	}
	if strings.ContainsAny(request.Password, "\r\n\x00") || len(request.Password) > maximumSteamPasswordLen {
		return fmt.Errorf("%w: password contains unsupported characters or is too long", ErrInvalidCredentials)
	}
	if passwordRequired && request.Password == "" {
		return fmt.Errorf("%w: password is required", ErrInvalidCredentials)
	}
	if strings.ContainsAny(request.SteamGuardCode, "\r\n\x00") || len(request.SteamGuardCode) > maximumSteamGuardLen {
		return fmt.Errorf("%w: Steam Guard code is invalid", ErrInvalidCredentials)
	}
	return nil
}

func (c *Client) ensureCredentialRuntime(ctx context.Context) error {
	if c.loginRunner != nil {
		return nil
	}
	return c.Ensure(ctx)
}

func (c *Client) executeExplicitLogin(ctx context.Context, request LoginRequest) error {
	if c.loginRunner == nil {
		return c.runExplicitLogin(ctx, request, nil)
	}
	commandCtx, cancel := c.commandContext(ctx)
	defer cancel()
	out, runErr := c.loginRunner(commandCtx, request)
	if authErr := explicitLoginFailure(out); authErr != nil {
		return authErr
	}
	if runErr != nil {
		return c.commandError(commandCtx, runErr, out, request.AccountName, request.Password, request.SteamGuardCode)
	}
	if err := commandCtx.Err(); err != nil {
		return fmt.Errorf("SteamCMD explicit login interrupted: %w", err)
	}
	return nil
}

func (c *Client) runExplicitLogin(ctx context.Context, request LoginRequest, commands []string) error {
	_, err := c.runExplicitLoginOutput(ctx, request, commands)
	return err
}

func (c *Client) runExplicitLoginOutput(ctx context.Context, request LoginRequest, commands []string) ([]byte, error) {
	release, err := c.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	if err := c.validateInstalled(); err != nil {
		return nil, fmt.Errorf("revalidate SteamCMD before explicit login: %w", err)
	}
	commandCtx, cancel := c.commandContext(ctx)
	defer cancel()

	lines := []string{"@ShutdownOnFailedCommand 1", "@NoPromptForPassword 1"}
	if len(commands) > 0 {
		lines = append(lines, "@sSteamCmdForcePlatformType windows")
	}
	loginLine := "login " + steamScriptArg(request.AccountName) + " " + steamScriptArg(request.Password)
	if request.SteamGuardCode != "" {
		loginLine += " " + steamScriptArg(request.SteamGuardCode)
	}
	lines = append(lines, loginLine)
	lines = append(lines, commands...)
	lines = append(lines, "quit")

	out, runErr := c.runCredentialScript(commandCtx, lines, request.Password, request.SteamGuardCode)
	if authErr := explicitLoginFailure(out); authErr != nil {
		return out, authErr
	}
	if runErr != nil {
		return out, c.commandError(commandCtx, runErr, out, request.AccountName, request.Password, request.SteamGuardCode)
	}
	if err := commandCtx.Err(); err != nil {
		return out, fmt.Errorf("SteamCMD explicit login interrupted: %w", err)
	}
	return out, nil
}

func (c *Client) runCachedLoginOutput(ctx context.Context, accountName string, beforeLogin, commands []string) ([]byte, error) {
	release, err := c.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	if err := c.validateInstalled(); err != nil {
		return nil, fmt.Errorf("revalidate SteamCMD before cached login: %w", err)
	}
	commandCtx, cancel := c.commandContext(ctx)
	defer cancel()

	lines := []string{"@ShutdownOnFailedCommand 1", "@NoPromptForPassword 1", "@sSteamCmdForcePlatformType windows"}
	lines = append(lines, beforeLogin...)
	lines = append(lines, "login "+steamScriptArg(accountName))
	lines = append(lines, commands...)
	lines = append(lines, "quit")
	out, runErr := c.runCredentialScript(commandCtx, lines)
	if cachedErr := cachedLoginFailure(out); cachedErr != nil {
		return out, cachedErr
	}
	if runErr != nil {
		return out, c.commandError(commandCtx, runErr, out, accountName)
	}
	if err := commandCtx.Err(); err != nil {
		return out, fmt.Errorf("SteamCMD cached login interrupted: %w", err)
	}
	return out, nil
}

func (c *Client) runCredentialScript(ctx context.Context, lines []string, secrets ...string) ([]byte, error) {
	directory := filepath.Dir(c.cfg.SteamWorkshopCredentialsPath())
	if err := c.validateManaged(directory); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create Steam credential directory: %w", err)
	}
	if err := c.removeStaleCredentialScripts(directory); err != nil {
		return nil, err
	}
	script, err := os.CreateTemp(directory, "steamcmd-login-*.txt")
	if err != nil {
		return nil, fmt.Errorf("create temporary SteamCMD login script: %w", err)
	}
	scriptPath := script.Name()
	defer func() { _ = os.Remove(scriptPath) }()
	if err := c.validateManaged(scriptPath); err != nil {
		_ = script.Close()
		return nil, err
	}
	if err := script.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		_ = script.Close()
		return nil, fmt.Errorf("secure temporary SteamCMD login script: %w", err)
	}
	writer := bufio.NewWriter(script)
	for _, line := range lines {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			_ = script.Close()
			return nil, fmt.Errorf("write temporary SteamCMD login script: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		_ = script.Close()
		return nil, err
	}
	if err := script.Sync(); err != nil {
		_ = script.Close()
		return nil, err
	}
	if err := script.Close(); err != nil {
		return nil, err
	}
	if err := securePrivatePath(ctx, scriptPath); err != nil {
		return nil, fmt.Errorf("secure temporary SteamCMD login script: %w", err)
	}
	return c.runConfiguredCommand(ctx, "+runscript", scriptPath)
}

func (c *Client) removeStaleCredentialScripts(directory string) error {
	paths, err := filepath.Glob(filepath.Join(directory, "steamcmd-login-*.txt"))
	if err != nil {
		return fmt.Errorf("list stale SteamCMD login scripts: %w", err)
	}
	for _, path := range paths {
		if err := c.validateManaged(path); err != nil {
			return err
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove stale SteamCMD login script: %w", err)
		}
	}
	return nil
}

func steamScriptArg(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func explicitLoginFailure(output []byte) error {
	lower := strings.ToLower(string(output))
	if strings.Contains(lower, "logged in ok") || strings.Contains(lower, "waiting for user info...ok") {
		return nil
	}
	mobileConfirmationMarkers := []string{
		"please confirm the login in the steam mobile app",
		"wait for confirmation timed out",
		"timed out waiting for confirmation",
	}
	for _, marker := range mobileConfirmationMarkers {
		if strings.Contains(lower, marker) {
			return ErrSteamMobileConfirmationRequired
		}
	}
	guardMarkers := []string{
		"need two-factor code",
		"two-factor code mismatch",
		"invalid authenticator code",
		"invalid login auth code",
		"invalid auth code",
		"steam guard code is invalid",
		"steam guard code is incorrect",
		"steam guard code required",
		"requires steam guard",
		"account logon denied, need two-factor",
	}
	for _, marker := range guardMarkers {
		if strings.Contains(lower, marker) {
			return ErrSteamGuardRequired
		}
	}
	credentialMarkers := []string{
		"invalid password",
		"password required",
		"failed to log in",
		"login failure",
		"account logon denied",
	}
	for _, marker := range credentialMarkers {
		if strings.Contains(lower, marker) {
			return ErrInvalidCredentials
		}
	}
	return nil
}

func cachedLoginFailure(output []byte) error {
	lower := strings.ToLower(string(output))
	for _, marker := range []string{
		"cached credentials not found",
		"no cached credentials",
		"cached credentials are missing or expired",
	} {
		if strings.Contains(lower, marker) {
			return ErrLoginRequired
		}
	}
	return explicitLoginFailure(output)
}

// CachedLoginFailure classifies stable authentication failures in SteamCMD
// output produced by either the native process or the Docker/Wine runner.
func CachedLoginFailure(output []byte) error {
	return cachedLoginFailure(output)
}

func (c *Client) readCredentials() (storedCredentials, bool, error) {
	path := c.cfg.SteamWorkshopCredentialsPath()
	if err := c.validateManaged(path); err != nil {
		return storedCredentials{}, false, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return storedCredentials{}, false, nil
	}
	if err != nil {
		return storedCredentials{}, false, fmt.Errorf("open Steam Workshop credentials: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, maximumCredentialBytes+1))
	decoder.DisallowUnknownFields()
	var credentials storedCredentials
	if err := decoder.Decode(&credentials); err != nil {
		return storedCredentials{}, false, fmt.Errorf("decode Steam Workshop credentials: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return storedCredentials{}, false, errors.New("Steam Workshop credential file contains trailing data")
	}
	if credentials.Version != steamCredentialVersion || credentials.Password == "" {
		return storedCredentials{}, false, errors.New("Steam Workshop credential file is invalid")
	}
	if err := ValidateAccountName(credentials.AccountName); err != nil {
		return storedCredentials{}, false, err
	}
	return credentials, true, nil
}

func (c *Client) writeCredentials(ctx context.Context, credentials storedCredentials) error {
	path := c.cfg.SteamWorkshopCredentialsPath()
	directory := filepath.Dir(path)
	if err := c.validateManaged(path); err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create Steam credential directory: %w", err)
	}
	body, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, "steam-workshop-credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary Steam credential file: %w", err)
	}
	temporaryPath := temporary.Name()
	complete := false
	defer func() {
		_ = temporary.Close()
		if !complete {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("secure temporary Steam credential file: %w", err)
	}
	if _, err := temporary.Write(body); err != nil {
		return fmt.Errorf("write Steam Workshop credentials: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := securePrivatePath(ctx, temporaryPath); err != nil {
		return fmt.Errorf("secure Steam credential file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		if runtime.GOOS != "windows" {
			return fmt.Errorf("replace Steam Workshop credentials: %w", err)
		}
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("replace Steam Workshop credentials: %w", err)
		}
		if retryErr := os.Rename(temporaryPath, path); retryErr != nil {
			return fmt.Errorf("replace Steam Workshop credentials: %w", retryErr)
		}
	}
	if err := securePrivatePath(ctx, path); err != nil {
		return fmt.Errorf("secure Steam Workshop credentials: %w", err)
	}
	complete = true
	return nil
}

func (c *Client) setSteamGuardRequired(required bool) {
	c.loginMu.Lock()
	c.login.steamGuardRequired = required
	c.loginMu.Unlock()
}
