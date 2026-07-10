package appconfig

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ParseEnvFile parses KEY=VALUE records without invoking a shell or expanding
// variables. That property is important because this file contains secrets.
func ParseEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(strings.TrimSuffix(scanner.Text(), "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			return nil, fmt.Errorf("%s:%d: export syntax is not allowed", path, lineNumber)
		}
		name, raw, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected KEY=VALUE", path, lineNumber)
		}
		name = strings.TrimSpace(name)
		if !envNamePattern.MatchString(name) {
			return nil, fmt.Errorf("%s:%d: invalid variable name %q", path, lineNumber, name)
		}
		if _, exists := values[name]; exists {
			return nil, fmt.Errorf("%s:%d: duplicate variable %s", path, lineNumber, name)
		}
		value, err := parseEnvValue(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
		}
		values[name] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func parseEnvValue(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if strings.ContainsRune(raw, 0) {
		return "", errors.New("NUL is not allowed")
	}
	if raw[0] == '\'' {
		if len(raw) < 2 || raw[len(raw)-1] != '\'' {
			return "", errors.New("unterminated single-quoted value")
		}
		return raw[1 : len(raw)-1], nil
	}
	if raw[0] == '"' {
		if len(raw) < 2 || raw[len(raw)-1] != '"' {
			return "", errors.New("unterminated double-quoted value")
		}
		value, err := strconv.Unquote(raw)
		if err != nil {
			return "", fmt.Errorf("invalid quoted value: %w", err)
		}
		return value, nil
	}
	if strings.ContainsAny(raw, "\"'") {
		return "", errors.New("quotes must surround the entire value")
	}
	return raw, nil
}

// ApplyFileEnvironment adds file values only where the process environment is
// absent. The returned function restores the previous environment.
func ApplyFileEnvironment(values map[string]string) (func(), error) {
	for name := range values {
		if !allowedFileEnvironmentName(name) {
			return nil, fmt.Errorf("configuration variable %s is not allowed", name)
		}
	}
	added := make([]string, 0, len(values))
	for name, value := range values {
		if _, exists := os.LookupEnv(name); exists {
			continue
		}
		_ = os.Setenv(name, value)
		added = append(added, name)
	}
	return func() {
		for _, name := range added {
			_ = os.Unsetenv(name)
		}
	}, nil
}

func allowedFileEnvironmentName(name string) bool {
	for _, prefix := range []string{"PALPANEL_", "PANEL_", "PALWORLD_", "STEAM_"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	switch name {
	case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "all_proxy", "no_proxy":
		return true
	default:
		return false
	}
}

func LoadFile(path string) (Config, error) {
	values, err := ParseEnvFile(path)
	if err != nil {
		return Config{}, err
	}
	restore, err := ApplyFileEnvironment(values)
	if err != nil {
		return Config{}, err
	}
	defer restore()
	return Load()
}

// InitFile creates a minimal production configuration. created is false when
// the file already exists; in that case no token is returned.
func InitFile(path string) (token string, created bool, err error) {
	path = filepath.Clean(path)
	if info, statErr := os.Stat(path); statErr == nil {
		if info.IsDir() {
			return "", false, fmt.Errorf("config path is a directory: %s", path)
		}
		if chmodErr := os.Chmod(path, 0o600); chmodErr != nil {
			return "", false, chmodErr
		}
		return "", false, nil
	} else if !os.IsNotExist(statErr) {
		return "", false, statErr
	}

	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", false, fmt.Errorf("generate panel token: %w", err)
	}
	token = hex.EncodeToString(random)
	body := strings.Join([]string{
		"# PalPanel production configuration. Parsed as data; shell syntax is not executed.",
		"PANEL_TOKEN=" + token,
		"PALPANEL_REQUIRE_AUTH=true",
		"PALPANEL_LISTEN_ADDR=127.0.0.1:8080",
		"PALPANEL_SAVE_INDEXER_ENABLED=true",
		"PALPANEL_SAVE_INDEXER_URL=http://127.0.0.1:8090",
		"PALPANEL_LOG_LEVEL=info",
		"",
	}, "\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil && filepath.Dir(path) != "." {
		return "", false, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", false, err
	}
	if _, err := file.WriteString(body); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", false, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", false, err
	}
	return token, true, nil
}
