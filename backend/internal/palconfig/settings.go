package palconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const SectionHeader = "[/Script/Pal.PalGameWorldSettings]"

type Settings map[string]string

func Read(path string) (Settings, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Settings{}, nil
	}
	if err != nil {
		return nil, err
	}
	return Parse(string(b))
}

func Write(path string, settings Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(Serialize(settings)), 0o644)
}

func Parse(content string) (Settings, error) {
	start := strings.Index(content, "OptionSettings=(")
	if start == -1 {
		return Settings{}, nil
	}
	start += len("OptionSettings=(")
	end := findClosingParen(content, start-1)
	if end == -1 {
		return nil, fmt.Errorf("invalid OptionSettings: missing closing parenthesis")
	}

	items := splitTopLevel(content[start:end])
	out := Settings{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		k, v, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = unquote(strings.TrimSpace(v))
	}
	return out, nil
}

func Serialize(settings Settings) string {
	keys := make([]string, 0, len(settings))
	for k := range settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+formatValue(settings[k]))
	}
	return SectionHeader + "\nOptionSettings=(" + strings.Join(parts, ",") + ")\n"
}

func Merge(current Settings, updates map[string]any) Settings {
	next := Settings{}
	for k, v := range current {
		next[k] = v
	}
	for k, v := range updates {
		next[k] = anyToString(v)
	}
	return next
}

func splitTopLevel(s string) []string {
	var out []string
	var b strings.Builder
	inQuote := false
	escape := false
	depth := 0
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
		case '(':
			if !inQuote {
				depth++
			}
			b.WriteRune(r)
		case ')':
			if !inQuote && depth > 0 {
				depth--
			}
			b.WriteRune(r)
		case ',':
			if !inQuote && depth == 0 {
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

func unquote(v string) string {
	if len(v) >= 2 && strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
		u, err := strconv.Unquote(v)
		if err == nil {
			return u
		}
	}
	return v
}

func formatValue(v string) string {
	if v == "" {
		return "\"\""
	}
	lower := strings.ToLower(v)
	if lower == "true" || lower == "false" || lower == "none" || lower == "item" || lower == "itemandequipment" || lower == "all" {
		return v
	}
	if strings.HasPrefix(v, "(") && strings.HasSuffix(v, ")") {
		return v
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return v
	}
	return strconv.Quote(v)
}

func anyToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "True"
		}
		return "False"
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return fmt.Sprint(t)
	}
}
