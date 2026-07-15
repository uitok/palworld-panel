//go:build !windows

package steamcmd

import "context"

func hardenCredentialTree(context.Context, string) error {
	return ErrInteractiveLogin
}

func launchInteractiveSteamCMD(string, string, string) error {
	return ErrInteractiveLogin
}
