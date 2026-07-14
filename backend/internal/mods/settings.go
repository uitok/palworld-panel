package mods

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const modHeader = "[PalModSettings]"

type Info struct {
	Name        string `json:"Name"`
	Version     string `json:"Version"`
	PackageName string `json:"PackageName"`
}

type ModSettings struct {
	GlobalEnabled bool
	ActiveMods    []string
}

func ReadInfo(path string) (Info, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Info{}, err
	}
	var info Info
	if err := json.Unmarshal(b, &info); err != nil {
		return Info{}, err
	}
	info.Name = strings.TrimSpace(info.Name)
	info.PackageName = strings.TrimSpace(info.PackageName)
	info.Version = strings.TrimSpace(info.Version)
	if info.PackageName == "" {
		return Info{}, fmt.Errorf("Info.json missing PackageName")
	}
	if len(info.PackageName) > 255 || strings.IndexFunc(info.PackageName, func(character rune) bool {
		return character < 0x20 || character == 0x7f
	}) >= 0 {
		return Info{}, fmt.Errorf("Info.json contains an invalid PackageName")
	}
	if info.Name == "" {
		info.Name = info.PackageName
	}
	return info, nil
}

func ReadModSettings(path string) (ModSettings, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ModSettings{GlobalEnabled: true}, nil
	}
	if err != nil {
		return ModSettings{}, err
	}
	return ParseModSettings(string(b)), nil
}

func WriteModSettings(path string, s ModSettings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	sort.Strings(s.ActiveMods)
	global := "false"
	if s.GlobalEnabled {
		global = "true"
	}
	var b strings.Builder
	b.WriteString(modHeader)
	b.WriteString("\n")
	b.WriteString("bGlobalEnableMod=")
	b.WriteString(global)
	b.WriteString("\n")
	for _, m := range s.ActiveMods {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		b.WriteString("ActiveModList=")
		b.WriteString(m)
		b.WriteString("\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func ParseModSettings(content string) ModSettings {
	out := ModSettings{GlobalEnabled: true}
	lower := strings.ToLower(content)
	if strings.Contains(lower, "bglobalenablemod=false") {
		out.GlobalEnabled = false
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "ActiveModList") {
			continue
		}
		value = strings.TrimSpace(value)
		if value != "" {
			out.ActiveMods = append(out.ActiveMods, value)
		}
	}
	if len(out.ActiveMods) > 0 {
		return out
	}
	key := "ActiveModList=("
	start := strings.Index(content, key)
	if start == -1 {
		return out
	}
	start += len(key)
	end := findClosingParen(content, start-1)
	if end == -1 {
		return out
	}
	for _, item := range splitTopLevel(content[start:end]) {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if u, err := strconv.Unquote(item); err == nil {
			item = u
		}
		out.ActiveMods = append(out.ActiveMods, item)
	}
	return out
}

func EnablePackage(s ModSettings, packageName string, enabled bool) ModSettings {
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		return s
	}
	seen := map[string]bool{}
	next := make([]string, 0, len(s.ActiveMods)+1)
	for _, m := range s.ActiveMods {
		if strings.EqualFold(m, packageName) {
			if enabled && !seen[packageName] {
				next = append(next, packageName)
				seen[packageName] = true
			}
			continue
		}
		if m != "" && !seen[m] {
			next = append(next, m)
			seen[m] = true
		}
	}
	if enabled && !seen[packageName] {
		next = append(next, packageName)
	}
	s.GlobalEnabled = true
	s.ActiveMods = next
	return s
}

func splitTopLevel(s string) []string {
	var out []string
	var b strings.Builder
	inQuote := false
	escape := false
	for _, r := range s {
		if escape {
			b.WriteRune(r)
			escape = false
			continue
		}
		switch r {
		case '\\':
			b.WriteRune(r)
			if inQuote {
				escape = true
			}
		case '"':
			inQuote = !inQuote
			b.WriteRune(r)
		case ',':
			if !inQuote {
				out = append(out, b.String())
				b.Reset()
			} else {
				b.WriteRune(r)
			}
		default:
			b.WriteRune(r)
		}
	}
	out = append(out, b.String())
	return out
}

func findClosingParen(s string, open int) int {
	depth := 0
	inQuote := false
	escape := false
	for i := open; i < len(s); i++ {
		ch := s[i]
		if escape {
			escape = false
			continue
		}
		if ch == '\\' && inQuote {
			escape = true
			continue
		}
		if ch == '"' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			continue
		}
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
