package paldefender

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"palpanel/internal/id"
)

var palTemplateNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

var palTemplateWorkTypes = map[string]bool{
	"BaseCampBattle": true, "EmitFlame": true, "Watering": true, "Seeding": true,
	"GenerateElectricity": true, "Handcraft": true, "Collection": true, "Deforest": true,
	"Mining": true, "OilExtraction": true, "ProductMedicine": true, "Cool": true,
	"Transport": true, "MonsterFarm": true,
}

type PalTemplate struct {
	PalID                  string         `json:"PalID"`
	UniqueNPCID            string         `json:"UniqueNPCID,omitempty"`
	Nickname               string         `json:"Nickname,omitempty"`
	SkinID                 string         `json:"SkinId,omitempty"`
	Gender                 string         `json:"Gender,omitempty"`
	Level                  *int           `json:"Level,omitempty"`
	EXP                    *int64         `json:"Exp,omitempty"`
	Shiny                  *bool          `json:"Shiny,omitempty"`
	PartnerSkillLevel      *int           `json:"PartnerSkillLevel,omitempty"`
	CondensedPals          *int           `json:"CondensedPals,omitempty"`
	UnusedStatusPoints     *int           `json:"UnusedStatusPoints,omitempty"`
	FriendshipPoints       *int64         `json:"FriendshipPoints,omitempty"`
	PhysicalHealth         string         `json:"PhysicalHealth,omitempty"`
	WorkerSick             string         `json:"WorkerSick,omitempty"`
	ImportedCharacter      *bool          `json:"ImportedCharacter,omitempty"`
	HP                     *float64       `json:"HP,omitempty"`
	SP                     *float64       `json:"SP,omitempty"`
	MP                     *float64       `json:"MP,omitempty"`
	Shield                 *float64       `json:"Shield,omitempty"`
	Hunger                 *float64       `json:"Hunger,omitempty"`
	MaxHunger              *float64       `json:"MaxHunger,omitempty"`
	SAN                    *float64       `json:"SAN,omitempty"`
	Support                *int           `json:"Support,omitempty"`
	CraftSpeed             *int           `json:"CraftSpeed,omitempty"`
	PalSouls               map[string]int `json:"PalSouls,omitempty"`
	IVs                    map[string]int `json:"IVs,omitempty"`
	ActiveSkills           []string       `json:"ActiveSkills,omitempty"`
	LearntSkills           []string       `json:"LearntSkills,omitempty"`
	Passives               []string       `json:"Passives,omitempty"`
	ExtraWorkSuitabilities map[string]int `json:"ExtraWorkSuitabilities,omitempty"`
	DisableWorkPreferences []string       `json:"DisableWorkPreferences,omitempty"`
}

type PalTemplateInfo struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

type ExportedPalTemplateInfo struct {
	PlayerID   string    `json:"player_id"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

func normalizePalTemplateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if strings.EqualFold(filepath.Ext(name), ".json") {
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	if !palTemplateNamePattern.MatchString(name) {
		return "", invalidRESTRequest("invalid Pal template name")
	}
	return name + ".json", nil
}

func (m Manager) palTemplatesDir() string {
	return filepath.Join(m.cfg.PalDefenderDir(), "Pals", "Templates")
}

func (m Manager) exportedPalsDir(identifier string) (string, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(m.cfg.PalDefenderDir(), "Pals", "Exported", identifier),
		filepath.Join(m.cfg.PalDefenderDir(), "Pals", "exported", identifier),
		filepath.Join(m.cfg.PalDefenderDir(), "pals", "Exported", identifier),
		filepath.Join(m.cfg.PalDefenderDir(), "pals", "exported", identifier),
	}
	dir := candidates[0]
	for _, candidate := range candidates {
		if dirExists(candidate) {
			dir = candidate
			break
		}
	}
	if err := m.cfg.ValidateManagedPath(dir, false); err != nil {
		return "", err
	}
	return dir, nil
}

func normalizeExportedPalTemplateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 255 || filepath.Base(name) != name || strings.ContainsAny(name, `/\`) || strings.ContainsRune(name, '\x00') || !strings.EqualFold(filepath.Ext(name), ".json") {
		return "", invalidRESTRequest("invalid exported Pal template name")
	}
	return name, nil
}

func (m Manager) palTemplatePath(name string) (string, error) {
	name, err := normalizePalTemplateName(name)
	if err != nil {
		return "", err
	}
	path := filepath.Join(m.palTemplatesDir(), name)
	if err := m.cfg.ValidateManagedPath(path, false); err != nil {
		return "", err
	}
	return path, nil
}

func (m Manager) ListPalTemplates() ([]PalTemplateInfo, error) {
	dir := m.palTemplatesDir()
	if err := m.cfg.ValidateManagedPath(dir, false); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []PalTemplateInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]PalTemplateInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		out = append(out, PalTemplateInfo{Name: entry.Name(), Path: filepath.Join(dir, entry.Name()), Size: info.Size(), ModifiedAt: info.ModTime().UTC()})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func (m Manager) ListExportedPalTemplates(identifier string) ([]ExportedPalTemplateInfo, error) {
	dir, err := m.exportedPalsDir(identifier)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []ExportedPalTemplateInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	identifier = strings.TrimSpace(identifier)
	out := make([]ExportedPalTemplateInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		out = append(out, ExportedPalTemplateInfo{
			PlayerID: identifier, Name: entry.Name(), Path: filepath.Join(dir, entry.Name()),
			Size: info.Size(), ModifiedAt: info.ModTime().UTC(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func (m Manager) ReadPalTemplate(name string) (PalTemplate, error) {
	path, err := m.palTemplatePath(name)
	if err != nil {
		return PalTemplate{}, err
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return PalTemplate{}, err
	}
	return decodePalTemplate(payload)
}

func (m Manager) ReadExportedPalTemplate(identifier, name string) (PalTemplate, error) {
	dir, err := m.exportedPalsDir(identifier)
	if err != nil {
		return PalTemplate{}, err
	}
	name, err = normalizeExportedPalTemplateName(name)
	if err != nil {
		return PalTemplate{}, err
	}
	path := filepath.Join(dir, name)
	if err := m.cfg.ValidateManagedPath(path, false); err != nil {
		return PalTemplate{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return PalTemplate{}, err
	}
	if !info.Mode().IsRegular() {
		return PalTemplate{}, invalidRESTRequest("exported Pal template is not a regular file")
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return PalTemplate{}, err
	}
	return decodePalTemplate(payload)
}

func decodePalTemplate(payload []byte) (PalTemplate, error) {
	var template PalTemplate
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&template); err != nil {
		return PalTemplate{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return PalTemplate{}, invalidRESTRequest("Pal template must contain exactly one JSON object")
		}
		return PalTemplate{}, err
	}
	if err := validatePalTemplate(&template); err != nil {
		return PalTemplate{}, err
	}
	return template, nil
}

func (m Manager) WritePalTemplate(name string, template PalTemplate) (PalTemplateInfo, error) {
	path, err := m.palTemplatePath(name)
	if err != nil {
		return PalTemplateInfo{}, err
	}
	if err := validatePalTemplate(&template); err != nil {
		return PalTemplateInfo{}, err
	}
	payload, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return PalTemplateInfo{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return PalTemplateInfo{}, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".pal-template-*.tmp")
	if err != nil {
		return PalTemplateInfo{}, err
	}
	temporaryPath := temporary.Name()
	complete := false
	defer func() {
		_ = temporary.Close()
		if !complete {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return PalTemplateInfo{}, err
	}
	if _, err := temporary.Write(append(payload, '\n')); err != nil {
		return PalTemplateInfo{}, err
	}
	if err := temporary.Sync(); err != nil {
		return PalTemplateInfo{}, err
	}
	if err := temporary.Close(); err != nil {
		return PalTemplateInfo{}, err
	}
	previous := path + ".palpanel-old-" + id.New("template")
	hadPrevious := fileExists(path)
	if hadPrevious {
		if err := os.Rename(path, previous); err != nil {
			return PalTemplateInfo{}, err
		}
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		if hadPrevious {
			_ = os.Rename(previous, path)
		}
		return PalTemplateInfo{}, err
	}
	_ = os.Remove(previous)
	complete = true
	info, err := os.Stat(path)
	if err != nil {
		return PalTemplateInfo{}, err
	}
	return PalTemplateInfo{Name: filepath.Base(path), Path: path, Size: info.Size(), ModifiedAt: info.ModTime().UTC()}, nil
}

func (m Manager) DeletePalTemplate(name string) error {
	path, err := m.palTemplatePath(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func validatePalTemplate(template *PalTemplate) error {
	template.PalID = strings.TrimSpace(template.PalID)
	if !itemIdentifierPattern.MatchString(template.PalID) {
		return invalidRESTRequest("PalID is required and must be a valid identifier")
	}
	for label, value := range map[string]*string{
		"UniqueNPCID": &template.UniqueNPCID, "SkinId": &template.SkinID,
	} {
		*value = strings.TrimSpace(*value)
		if *value != "" && !itemIdentifierPattern.MatchString(*value) {
			return invalidRESTRequest("%s must be a valid identifier", label)
		}
	}
	template.Nickname = strings.TrimSpace(template.Nickname)
	if len([]rune(template.Nickname)) > 64 || strings.ContainsRune(template.Nickname, '\x00') {
		return invalidRESTRequest("Nickname must not exceed 64 characters")
	}
	if template.Gender != "" {
		switch strings.ToLower(strings.TrimSpace(template.Gender)) {
		case "male":
			template.Gender = "Male"
		case "female":
			template.Gender = "Female"
		case "none":
			template.Gender = "None"
		}
		if template.Gender != "Male" && template.Gender != "Female" && template.Gender != "None" {
			return invalidRESTRequest("Gender must be Male, Female, or None")
		}
	}
	if err := validateOptionalInt("Level", template.Level, 1, 255); err != nil {
		return err
	}
	if err := validateOptionalInt("PartnerSkillLevel", template.PartnerSkillLevel, 1, 255); err != nil {
		return err
	}
	for name, value := range map[string]*int{
		"CondensedPals": template.CondensedPals, "UnusedStatusPoints": template.UnusedStatusPoints,
		"Support": template.Support, "CraftSpeed": template.CraftSpeed,
	} {
		if err := validateOptionalInt(name, value, 0, 2_147_483_647); err != nil {
			return err
		}
	}
	for name, value := range map[string]*int64{"Exp": template.EXP, "FriendshipPoints": template.FriendshipPoints} {
		if value != nil && (*value < 0 || *value > maxProgressionGrant) {
			return invalidRESTRequest("%s must be between 0 and %d", name, maxProgressionGrant)
		}
	}
	for name, value := range map[string]*float64{
		"HP": template.HP, "SP": template.SP, "MP": template.MP, "Shield": template.Shield,
		"Hunger": template.Hunger, "MaxHunger": template.MaxHunger, "SAN": template.SAN,
	} {
		if value != nil && (*value < 0 || *value > float64(maxProgressionGrant)) {
			return invalidRESTRequest("%s must be between 0 and %d", name, maxProgressionGrant)
		}
	}
	physicalStates := map[string]bool{"": true, "Healthful": true, "MinorInjury": true, "Severe": true, "Dying": true, "DeadBody": true, "CloudCemetery": true}
	if !physicalStates[template.PhysicalHealth] {
		return invalidRESTRequest("unsupported PhysicalHealth value")
	}
	workerStates := map[string]bool{"": true, "None": true, "Cold": true, "Sprain": true, "Bulimia": true, "GastricUlcer": true, "Fracture": true, "Weakness": true, "DepressionSprain": true, "DisturbingElement": true}
	if !workerStates[template.WorkerSick] {
		return invalidRESTRequest("unsupported WorkerSick value")
	}
	if err := validateRankMap("PalSouls", template.PalSouls, map[string]bool{"Health": true, "Attack": true, "Defense": true, "CraftSpeed": true}); err != nil {
		return err
	}
	if err := validateRankMap("IVs", template.IVs, map[string]bool{"Health": true, "AttackMelee": true, "AttackShot": true, "Defense": true}); err != nil {
		return err
	}
	for name, list := range map[string][]string{"ActiveSkills": template.ActiveSkills, "LearntSkills": template.LearntSkills, "Passives": template.Passives} {
		if err := validateIdentifierList(name, list, 64); err != nil {
			return err
		}
	}
	if len(template.ActiveSkills) > 3 {
		return invalidRESTRequest("ActiveSkills must contain at most 3 equipped skills")
	}
	for workType, rank := range template.ExtraWorkSuitabilities {
		if (!palTemplateWorkTypes[workType] && workType != "Anyone") || rank < 0 || rank > 255 {
			return invalidRESTRequest("invalid ExtraWorkSuitabilities entry %q", workType)
		}
	}
	for _, workType := range template.DisableWorkPreferences {
		if !palTemplateWorkTypes[workType] {
			return invalidRESTRequest("invalid DisableWorkPreferences entry %q", workType)
		}
	}
	return nil
}

func validateOptionalInt(name string, value *int, minimum, maximum int) error {
	if value != nil && (*value < minimum || *value > maximum) {
		return invalidRESTRequest("%s must be between %d and %d", name, minimum, maximum)
	}
	return nil
}

func validateRankMap(name string, values map[string]int, allowed map[string]bool) error {
	for key, value := range values {
		if !allowed[key] || value < 0 || value > 255 {
			return invalidRESTRequest("invalid %s entry %q", name, key)
		}
	}
	return nil
}

func validateIdentifierList(name string, values []string, maximum int) error {
	if len(values) > maximum {
		return invalidRESTRequest("%s must contain at most %d identifiers", name, maximum)
	}
	for index, value := range values {
		if !itemIdentifierPattern.MatchString(strings.TrimSpace(value)) {
			return invalidRESTRequest("%s entry %d is invalid", name, index+1)
		}
	}
	return nil
}
