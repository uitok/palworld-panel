package steamcmd

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateAccountNameRejectsSteamCMDArgumentInjection(t *testing.T) {
	for _, value := range []string{"", "ab", "user name", "user+quit", "+login", "user@example.com", "用户", strings.Repeat("a", 65)} {
		if err := ValidateAccountName(value); !errors.Is(err, ErrInvalidAccountName) {
			t.Errorf("ValidateAccountName(%q) = %v", value, err)
		}
	}
	for _, value := range []string{"abc", "Fixture_User_123", strings.Repeat("a", 64)} {
		if err := ValidateAccountName(value); err != nil {
			t.Errorf("ValidateAccountName(%q) = %v", value, err)
		}
	}
}
