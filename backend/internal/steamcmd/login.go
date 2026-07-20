package steamcmd

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrLoginRequired                   = errors.New("Steam credentials are required before downloading Workshop Mods")
	ErrInteractiveLogin                = errors.New("explicit SteamCMD login is unavailable for the current runtime")
	ErrInvalidAccountName              = errors.New("invalid Steam account name")
	ErrInvalidCredentials              = errors.New("invalid Steam credentials")
	ErrSteamGuardRequired              = errors.New("Steam Guard verification code is required")
	ErrSteamMobileConfirmationRequired = errors.New("Steam Mobile login confirmation is required")
	steamAccountNameRegex              = regexp.MustCompile(`^[A-Za-z0-9_]{3,64}$`)
)

// LoginStatus contains only non-secret account and verification metadata.
// Passwords and Steam Guard codes never enter this type.
type LoginStatus struct {
	Supported            bool   `json:"supported"`
	SteamCMDInstalled    bool   `json:"steamcmd_installed"`
	CredentialsSecure    bool   `json:"credentials_secure"`
	LoginInProgress      bool   `json:"login_in_progress"`
	LoggedIn             bool   `json:"logged_in"`
	VerificationRequired bool   `json:"verification_required"`
	AccountName          string `json:"account_name,omitempty"`
	LastVerifiedAt       string `json:"last_verified_at,omitempty"`
	PasswordConfigured   bool   `json:"password_configured"`
	SteamGuardRequired   bool   `json:"steam_guard_required"`
	Message              string `json:"message,omitempty"`
}

// LoginRequest contains credentials submitted for one explicit SteamCMD
// authentication attempt. SteamGuardCode is deliberately transient and is
// never written to the credential store.
type LoginRequest struct {
	AccountName    string
	Password       string
	SteamGuardCode string
}

type loginState struct {
	steamGuardRequired bool
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
