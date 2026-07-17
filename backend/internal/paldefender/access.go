package paldefender

import (
	"fmt"
	"strings"
)

type AccessSettings struct {
	UseWhitelist      bool     `json:"use_whitelist"`
	WhitelistMessage  string   `json:"whitelist_message"`
	UseAdminWhitelist bool     `json:"use_admin_whitelist"`
	AdminAutoLogin    bool     `json:"admin_auto_login"`
	AdminIPs          []string `json:"admin_ips"`
	ReloadRequired    bool     `json:"reload_required"`
	ReferenceURL      string   `json:"reference_url"`
}

type AccessSettingsUpdate struct {
	UseWhitelist      bool     `json:"use_whitelist"`
	WhitelistMessage  string   `json:"whitelist_message"`
	UseAdminWhitelist bool     `json:"use_admin_whitelist"`
	AdminAutoLogin    bool     `json:"admin_auto_login"`
	AdminIPs          []string `json:"admin_ips"`
}

func (m Manager) ReadAccessSettings() (AccessSettings, error) {
	config, err := m.ReadConfig()
	if err != nil {
		return AccessSettings{}, err
	}
	return accessSettingsFromConfig(config), nil
}

func (m Manager) WriteAccessSettings(update AccessSettingsUpdate) (AccessSettings, error) {
	update.WhitelistMessage = strings.TrimSpace(update.WhitelistMessage)
	if len([]rune(update.WhitelistMessage)) > 512 || strings.ContainsRune(update.WhitelistMessage, '\x00') {
		return AccessSettings{}, invalidRESTRequest("whitelist_message must not exceed 512 characters")
	}
	if update.UseWhitelist && update.WhitelistMessage == "" {
		update.WhitelistMessage = "This server uses a whitelist."
	}
	adminIPs := make([]string, 0, len(update.AdminIPs))
	seen := map[string]bool{}
	for _, raw := range update.AdminIPs {
		value, err := validateAdminIP(raw)
		if err != nil {
			return AccessSettings{}, err
		}
		key := strings.ToLower(value)
		if !seen[key] {
			seen[key] = true
			adminIPs = append(adminIPs, value)
		}
	}
	if update.UseAdminWhitelist && len(adminIPs) == 0 {
		return AccessSettings{}, invalidRESTRequest("admin_ips must contain at least one address when use_admin_whitelist is enabled")
	}
	config, err := m.ReadConfig()
	if err != nil {
		return AccessSettings{}, err
	}
	config["useWhitelist"] = update.UseWhitelist
	config["whitelistMessage"] = update.WhitelistMessage
	config["useAdminWhitelist"] = update.UseAdminWhitelist
	config["adminAutoLogin"] = update.AdminAutoLogin
	config["adminIPs"] = adminIPs
	if _, err := m.WriteConfig(config); err != nil {
		return AccessSettings{}, err
	}
	settings := accessSettingsFromConfig(config)
	settings.ReloadRequired = true
	return settings, nil
}

func accessSettingsFromConfig(config map[string]any) AccessSettings {
	return AccessSettings{
		UseWhitelist:      boolFromAny(config["useWhitelist"]),
		WhitelistMessage:  strings.TrimSpace(stringValue(config["whitelistMessage"])),
		UseAdminWhitelist: boolFromAny(config["useAdminWhitelist"]),
		AdminAutoLogin:    boolFromAny(config["adminAutoLogin"]),
		AdminIPs:          stringSliceFromAny(config["adminIPs"]),
		ReferenceURL:      "https://github.com/Ultimeit/PalDefender/blob/main/docs/zh/FileTypes/Config.md",
	}
}

func boolFromAny(value any) bool {
	result, _ := value.(bool)
	return result
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func stringSliceFromAny(value any) []string {
	var out []string
	switch items := value.(type) {
	case []string:
		out = append(out, items...)
	case []any:
		for _, item := range items {
			if text := strings.TrimSpace(stringValue(item)); text != "" {
				out = append(out, text)
			}
		}
	}
	if out == nil {
		return []string{}
	}
	return out
}
