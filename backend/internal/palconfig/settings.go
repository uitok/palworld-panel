package palconfig

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const SectionHeader = "[/Script/Pal.PalGameWorldSettings]"

type Settings map[string]string

type Document struct {
	Settings  Settings
	RawValues map[string]string
}

type FormatIssue struct {
	Field    string `json:"field"`
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func Read(path string) (Settings, error) {
	document, err := ReadDocument(path)
	return document.Settings, err
}

func ReadDocument(path string) (Document, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Document{Settings: Settings{}, RawValues: map[string]string{}}, nil
	}
	if err != nil {
		return Document{}, err
	}
	return ParseDocument(string(b))
}

func Write(path string, settings Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(Serialize(settings)), 0o644)
}

func Parse(content string) (Settings, error) {
	document, err := ParseDocument(content)
	return document.Settings, err
}

func ParseDocument(content string) (Document, error) {
	start := strings.Index(content, "OptionSettings=(")
	if start == -1 {
		return Document{Settings: Settings{}, RawValues: map[string]string{}}, nil
	}
	start += len("OptionSettings=(")
	end := findClosingParen(content, start-1)
	if end == -1 {
		return Document{}, fmt.Errorf("invalid OptionSettings: missing closing parenthesis")
	}

	items := splitTopLevel(content[start:end])
	out := Settings{}
	rawValues := map[string]string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		k, v, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		key := strings.TrimSpace(k)
		raw := strings.TrimSpace(v)
		out[key] = unquote(raw)
		rawValues[key] = raw
	}
	return Document{Settings: out, RawValues: rawValues}, nil
}

func Serialize(settings Settings) string {
	return serialize(settings, nil, nil)
}

func SerializeDocument(document Document, modified map[string]bool) string {
	return serialize(document.Settings, document.RawValues, modified)
}

func serialize(settings Settings, rawValues map[string]string, modified map[string]bool) string {
	keys := make([]string, 0, len(settings))
	for k := range settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		value := ""
		raw, rawExists := rawValues[k]
		if _, known := schemaField(k); !known && !modified[k] && rawValues != nil && rawExists {
			value = raw
		} else {
			value = formatFieldValue(k, settings[k])
		}
		parts = append(parts, k+"="+value)
	}
	return SectionHeader + "\nOptionSettings=(" + strings.Join(parts, ",") + ")\n"
}

func FormatIssues(document Document) []FormatIssue {
	issues := []FormatIssue{}
	for _, key := range []string{"AdminPassword", "ServerPassword"} {
		raw, exists := document.RawValues[key]
		if !exists || isQuoted(raw) {
			continue
		}
		issues = append(issues, FormatIssue{
			Field: key, Code: "string_not_quoted", Severity: "warning",
			Message: key + " uses an unquoted string value and will be repaired when the draft is applied",
		})
	}
	return issues
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

func formatFieldValue(key, value string) string {
	field, known := schemaField(key)
	if !known {
		return formatValue(value)
	}
	switch field.Type {
	case TypeString:
		return strconv.Quote(value)
	case TypeBool:
		if strings.EqualFold(value, "true") {
			return "True"
		}
		return "False"
	case TypeInt:
		if number, ok := parseExactInt64(value); ok {
			return strconv.FormatInt(number, 10)
		}
	case TypeFloat:
		if number, err := strconv.ParseFloat(value, 64); err == nil {
			return strconv.FormatFloat(number, 'f', -1, 64)
		}
	case TypeEnum:
		for _, option := range field.Enum {
			if strings.EqualFold(option, value) {
				return option
			}
		}
	case TypeList:
		if normalized, err := normalizeStructuredList(value); err == nil {
			return normalized
		}
	}
	return formatValue(value)
}

func NormalizeFieldValue(key, value string) (string, error) {
	field, known := schemaField(key)
	if !known {
		return "", fmt.Errorf("unknown Palworld setting %s", key)
	}
	if issue := validateField(field, value); issue != nil {
		return "", fmt.Errorf("invalid Palworld setting %s", key)
	}
	return formatFieldValue(key, value), nil
}

func parseExactInt64(value string) (int64, bool) {
	if number, err := strconv.ParseInt(value, 10, 64); err == nil {
		return number, true
	}
	rational, ok := new(big.Rat).SetString(value)
	if !ok || !rational.IsInt() || !rational.Num().IsInt64() {
		return 0, false
	}
	return rational.Num().Int64(), true
}

func normalizeStructuredList(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len(value) < 2 || value[0] != '(' || value[len(value)-1] != ')' {
		return "", fmt.Errorf("list must use one outer pair of parentheses")
	}
	inner := value[1 : len(value)-1]
	if strings.TrimSpace(inner) == "" {
		return "()", nil
	}
	items := []string{}
	var token strings.Builder
	inQuote := false
	escape := false
	flush := func() error {
		raw := strings.TrimSpace(token.String())
		token.Reset()
		if raw == "" {
			return fmt.Errorf("list item is empty")
		}
		if strings.HasPrefix(raw, `"`) {
			decoded, err := strconv.Unquote(raw)
			if err != nil {
				return fmt.Errorf("invalid quoted list item")
			}
			items = append(items, strconv.Quote(decoded))
			return nil
		}
		for _, r := range raw {
			if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_-.:", r)) {
				return fmt.Errorf("invalid unquoted list item")
			}
		}
		items = append(items, raw)
		return nil
	}
	for _, r := range inner {
		if escape {
			token.WriteRune(r)
			escape = false
			continue
		}
		if inQuote && r == '\\' {
			token.WriteRune(r)
			escape = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			token.WriteRune(r)
			continue
		}
		if !inQuote && r == ',' {
			if err := flush(); err != nil {
				return "", err
			}
			continue
		}
		if !inQuote && (r == '(' || r == ')' || r == '=' || r == '\n' || r == '\r') {
			return "", fmt.Errorf("unsafe list syntax")
		}
		token.WriteRune(r)
	}
	if inQuote || escape {
		return "", fmt.Errorf("unterminated quoted list item")
	}
	if err := flush(); err != nil {
		return "", err
	}
	return "(" + strings.Join(items, ",") + ")", nil
}

func schemaField(key string) (FieldSchema, bool) {
	for _, field := range Schema() {
		if field.Key == key {
			return field, true
		}
	}
	return FieldSchema{}, false
}

func isQuoted(value string) bool {
	return len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)
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
