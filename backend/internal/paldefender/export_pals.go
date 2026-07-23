package paldefender

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	ErrExportPlayerUnavailable = errors.New("player is offline or not loaded")
	ErrExportNoPals            = errors.New("player has no Pals to export")
	ErrExportCommandRejected   = errors.New("PalDefender rejected the export command")
	ErrExportPalsTimeout       = errors.New("PalDefender did not generate a new export file within 30 seconds")
)

type ExportPalsResult struct {
	PlayerID     string                    `json:"player_id"`
	Command      string                    `json:"command"`
	Output       string                    `json:"output"`
	Templates    []ExportedPalTemplateInfo `json:"templates"`
	TemplateInfo ExportedPalTemplateInfo   `json:"template_info"`
	Template     PalTemplate               `json:"template"`
}

type exportedPalFileState struct {
	Size       int64
	ModifiedAt time.Time
}

func (m Manager) ExportPals(ctx context.Context, identifier string) (ExportPalsResult, error) {
	player, err := m.resolveExportPlayer(ctx, identifier)
	if err != nil {
		return ExportPalsResult{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(player.Status), "online") {
		return ExportPalsResult{}, ErrExportPlayerUnavailable
	}
	before, err := m.exportedPalSnapshot(player.UserID)
	if err != nil {
		return ExportPalsResult{}, err
	}
	rcon := m.exportRCON
	if rcon == nil {
		rcon = m.RCONExportPals
	}
	command, err := rcon(ctx, player.UserID)
	if err != nil {
		return ExportPalsResult{}, err
	}
	if err := classifyExportPalsOutput(command.Output); err != nil {
		return ExportPalsResult{}, err
	}

	templates, err := m.waitForFreshExportedPals(ctx, player.UserID, before)
	if err != nil {
		return ExportPalsResult{}, err
	}
	sort.Slice(templates, func(left, right int) bool {
		return templates[left].ModifiedAt.After(templates[right].ModifiedAt)
	})
	latest := templates[0]
	template, err := m.readExportedPalTemplateFile(latest.Path)
	if err != nil {
		return ExportPalsResult{}, fmt.Errorf("read exported Pal template: %w", err)
	}
	return ExportPalsResult{
		PlayerID: player.UserID, Command: command.Command, Output: command.Output,
		Templates: templates, TemplateInfo: latest, Template: template,
	}, nil
}

func (m Manager) resolveExportPlayer(ctx context.Context, identifier string) (RESTPlayer, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return RESTPlayer{}, err
	}
	players, err := m.RESTPlayers(ctx)
	if err != nil {
		return RESTPlayer{}, err
	}
	key := comparablePlayerIdentifier(identifier)
	for _, player := range players.Players {
		if comparablePlayerIdentifier(player.UserID) == key || comparablePlayerIdentifier(player.PlayerUID) == key {
			if strings.TrimSpace(player.UserID) == "" {
				return RESTPlayer{}, invalidRESTResponse("player payload did not contain a PalDefender UserId")
			}
			return player, nil
		}
	}
	return RESTPlayer{}, &RESTError{Status: 404, Code: "PLAYER_NOT_FOUND", Message: "player was not found"}
}

func comparablePlayerIdentifier(value string) string {
	var normalized strings.Builder
	for _, char := range strings.ToLower(strings.TrimSpace(value)) {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			normalized.WriteRune(char)
		}
	}
	return normalized.String()
}

func (m Manager) exportedPalSnapshot(identifier string) (map[string]exportedPalFileState, error) {
	files, err := m.ListExportedPalTemplates(identifier)
	if err != nil {
		return nil, err
	}
	snapshot := make(map[string]exportedPalFileState, len(files))
	for _, file := range files {
		snapshot[strings.ToLower(file.Name)] = exportedPalFileState{Size: file.Size, ModifiedAt: file.ModifiedAt}
	}
	return snapshot, nil
}

func (m Manager) waitForFreshExportedPals(ctx context.Context, identifier string, before map[string]exportedPalFileState) ([]ExportedPalTemplateInfo, error) {
	pollInterval := m.exportPollInterval
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	timeout := m.exportTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		files, err := m.ListExportedPalTemplates(identifier)
		if err != nil {
			return nil, err
		}
		fresh := make([]ExportedPalTemplateInfo, 0, len(files))
		for _, file := range files {
			previous, existed := before[strings.ToLower(file.Name)]
			if !existed || previous.Size != file.Size || !previous.ModifiedAt.Equal(file.ModifiedAt) {
				fresh = append(fresh, file)
			}
		}
		if len(fresh) > 0 {
			return fresh, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, ErrExportPalsTimeout
		case <-ticker.C:
		}
	}
}

func classifyExportPalsOutput(output string) error {
	lower := strings.ToLower(strings.TrimSpace(output))
	switch {
	case containsAny(lower, "offline", "not online", "not loaded", "player not found", "离线", "未加载", "找不到玩家"):
		return ErrExportPlayerUnavailable
	case containsAny(lower, "no pals", "0 pals", "does not have any pal", "没有帕鲁", "无帕鲁"):
		return ErrExportNoPals
	case containsAny(lower, "rejected", "denied", "unknown command", "permission", "failed", "拒绝", "无权限", "未知命令", "失败"):
		return ErrExportCommandRejected
	default:
		return nil
	}
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
