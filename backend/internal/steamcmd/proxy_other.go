//go:build !windows

package steamcmd

import (
	"context"

	"palpanel/internal/appconfig"
)

func withSteamCMDProxy(_ context.Context, _, _ string, run func() ([]byte, error)) ([]byte, error) {
	return run()
}

func RecoverProxyOverride(appconfig.Config) error { return nil }
