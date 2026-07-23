//go:build windows

package steamcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/windows/registry"

	"palpanel/internal/appconfig"
	"palpanel/internal/networkproxy"
)

const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

var steamCMDProxyMu sync.Mutex

type proxyRegistrySnapshot struct {
	Version       int            `json:"version"`
	ProxyEnable   optionalDWORD  `json:"proxy_enable"`
	ProxyServer   optionalString `json:"proxy_server"`
	ProxyOverride optionalString `json:"proxy_override"`
	AutoConfigURL optionalString `json:"auto_config_url"`
	AutoDetect    optionalDWORD  `json:"auto_detect"`
}

type optionalDWORD struct {
	Exists bool   `json:"exists"`
	Value  uint32 `json:"value,omitempty"`
}

type optionalString struct {
	Exists bool   `json:"exists"`
	Value  string `json:"value,omitempty"`
}

func withSteamCMDProxy(_ context.Context, rawProxy, markerPath string, run func() ([]byte, error)) ([]byte, error) {
	steamCMDProxyMu.Lock()
	defer steamCMDProxyMu.Unlock()
	bridge, err := networkproxy.StartBridge(rawProxy)
	if err != nil {
		return nil, err
	}
	defer bridge.Close()
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return nil, errors.New("cannot open current-user Internet proxy settings")
	}
	defer key.Close()
	snapshot := captureProxyRegistry(key)
	if err := writeProxyRestoreMarker(markerPath, snapshot); err != nil {
		return nil, err
	}
	if err := applyProxyRegistry(key, bridge.Address()); err != nil {
		_ = restoreProxyRegistry(key, snapshot)
		_ = os.Remove(markerPath)
		return nil, err
	}
	output, runErr := run()
	restoreErr := restoreProxyRegistry(key, snapshot)
	if restoreErr == nil {
		_ = os.Remove(markerPath)
	}
	if restoreErr != nil {
		if runErr != nil {
			return output, fmt.Errorf("%w; current-user proxy restoration also failed", runErr)
		}
		return output, errors.New("SteamCMD finished but current-user proxy settings could not be restored")
	}
	return output, runErr
}

// RecoverProxyOverride restores a snapshot left behind by a process crash.
// It is called once during PalPanel startup, before any SteamCMD command runs.
func RecoverProxyOverride(cfg appconfig.Config) error {
	steamCMDProxyMu.Lock()
	defer steamCMDProxyMu.Unlock()
	path := cfg.SteamCMDProxyRestorePath()
	if err := cfg.ValidateManagedPath(path, false); err != nil {
		return errors.New("SteamCMD proxy restoration marker is outside the managed runtime")
	}
	if info, err := os.Stat(path); err == nil && info.Size() > 32<<10 {
		return errors.New("SteamCMD proxy restoration marker is invalid")
	}
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return errors.New("cannot read SteamCMD proxy restoration marker")
	}
	var snapshot proxyRegistrySnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil || snapshot.Version != 1 {
		return errors.New("SteamCMD proxy restoration marker is invalid")
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return errors.New("cannot open current-user Internet proxy settings for recovery")
	}
	defer key.Close()
	if err := restoreProxyRegistry(key, snapshot); err != nil {
		return errors.New("cannot recover current-user Internet proxy settings")
	}
	return os.Remove(path)
}

func captureProxyRegistry(key registry.Key) proxyRegistrySnapshot {
	return proxyRegistrySnapshot{
		Version:     1,
		ProxyEnable: readDWORD(key, "ProxyEnable"), ProxyServer: readString(key, "ProxyServer"),
		ProxyOverride: readString(key, "ProxyOverride"), AutoConfigURL: readString(key, "AutoConfigURL"),
		AutoDetect: readDWORD(key, "AutoDetect"),
	}
}

func applyProxyRegistry(key registry.Key, address string) error {
	if address == "" {
		return errors.New("local SteamCMD proxy bridge did not provide an address")
	}
	proxyValue := "http=" + address + ";https=" + address
	if err := key.SetDWordValue("ProxyEnable", 1); err != nil {
		return errors.New("cannot enable current-user proxy for SteamCMD")
	}
	if err := key.SetStringValue("ProxyServer", proxyValue); err != nil {
		return errors.New("cannot configure current-user proxy for SteamCMD")
	}
	if err := key.SetStringValue("ProxyOverride", "<local>;localhost;127.*;[::1]"); err != nil {
		return errors.New("cannot configure current-user proxy exclusions")
	}
	if err := key.SetDWordValue("AutoDetect", 0); err != nil {
		return errors.New("cannot disable automatic proxy detection for SteamCMD")
	}
	if err := key.DeleteValue("AutoConfigURL"); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return errors.New("cannot suspend automatic proxy script for SteamCMD")
	}
	return nil
}

func restoreProxyRegistry(key registry.Key, snapshot proxyRegistrySnapshot) error {
	for name, value := range map[string]optionalDWORD{"ProxyEnable": snapshot.ProxyEnable, "AutoDetect": snapshot.AutoDetect} {
		if value.Exists {
			if err := key.SetDWordValue(name, value.Value); err != nil {
				return err
			}
		} else if err := key.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
	}
	for name, value := range map[string]optionalString{"ProxyServer": snapshot.ProxyServer, "ProxyOverride": snapshot.ProxyOverride, "AutoConfigURL": snapshot.AutoConfigURL} {
		if value.Exists {
			if err := key.SetStringValue(name, value.Value); err != nil {
				return err
			}
		} else if err := key.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
	}
	return nil
}

func readDWORD(key registry.Key, name string) optionalDWORD {
	value, _, err := key.GetIntegerValue(name)
	if err != nil {
		return optionalDWORD{}
	}
	return optionalDWORD{Exists: true, Value: uint32(value)}
}

func readString(key registry.Key, name string) optionalString {
	value, _, err := key.GetStringValue(name)
	if err != nil {
		return optionalString{}
	}
	return optionalString{Exists: true, Value: value}
}

func writeProxyRestoreMarker(path string, snapshot proxyRegistrySnapshot) error {
	if path == "" {
		return errors.New("SteamCMD proxy restoration path is not initialized")
	}
	if _, err := os.Stat(path); err == nil {
		return errors.New("a previous SteamCMD proxy restoration is still pending; restart PalPanel before retrying")
	} else if !os.IsNotExist(err) {
		return errors.New("cannot inspect SteamCMD proxy restoration marker")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return errors.New("cannot create SteamCMD proxy restoration directory")
	}
	body, err := json.Marshal(snapshot)
	if err != nil {
		return errors.New("cannot encode SteamCMD proxy restoration marker")
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".steamcmd-proxy-restore-*.tmp")
	if err != nil {
		return errors.New("cannot create SteamCMD proxy restoration marker")
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return errors.New("cannot secure SteamCMD proxy restoration marker")
	}
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return errors.New("cannot write SteamCMD proxy restoration marker")
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return errors.New("cannot sync SteamCMD proxy restoration marker")
	}
	if err := temporary.Close(); err != nil {
		return errors.New("cannot close SteamCMD proxy restoration marker")
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return errors.New("cannot activate SteamCMD proxy restoration marker")
	}
	return nil
}
