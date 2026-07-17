package paldefender

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

const (
	maxTechnologySelections = 500
	maxPalGrants            = 100
	maxPalTemplateGrants    = 20
	maxProgressionGrant     = int64(2_147_483_647)
)

var progressionRelicTypes = map[string]bool{
	"CapturePower": true, "HungerReduction": true, "SwimSpeed": true,
	"FoodDecayReduction": true, "JumpPower": true, "GliderSpeed": true,
	"ClimbSpeed": true, "StatusAilmentResist": true, "StaminaReduction": true,
	"SphereHoming": true, "ExpBonus": true, "RainbowPassiveRate": true,
	"MoveSpeed": true,
}

type RESTProgressionMeta struct {
	PlayerUID string `json:"PlayerUID"`
	Player    string `json:"Player"`
}

type RESTProgressionPlayer struct {
	Level              int64 `json:"level"`
	EXP                int64 `json:"exp"`
	UnusedStatusPoints int64 `json:"unusedStatusPoints"`
}

type RESTProgressionCurrencies struct {
	Relics                  map[string]int64 `json:"relics"`
	TechnologyPoints        int64            `json:"technologyPoints"`
	AncientTechnologyPoints int64            `json:"ancientTechnologyPoints"`
}

type RESTProgression struct {
	Player     RESTProgressionPlayer     `json:"Player"`
	Currencies RESTProgressionCurrencies `json:"Currencies"`
	Bosses     map[string]any            `json:"Bosses"`
	Captures   map[string]any            `json:"Captures"`
	Activities map[string]any            `json:"Activities"`
}

type RESTProgressionResponse struct {
	Meta        RESTProgressionMeta `json:"Meta"`
	Progression RESTProgression     `json:"Progression"`
}

type GiveProgressionRequest struct {
	EXP                     *int64           `json:"EXP,omitempty"`
	Relics                  map[string]int64 `json:"Relics,omitempty"`
	TechnologyPoints        *int64           `json:"TechnologyPoints,omitempty"`
	AncientTechnologyPoints *int64           `json:"AncientTechnologyPoints,omitempty"`
}

type GiveProgressionResponse struct {
	Granted map[string]any `json:"Granted"`
	Totals  map[string]any `json:"Totals"`
}

type RESTTechsMeta struct {
	PlayerUID     string `json:"PlayerUID"`
	Player        string `json:"Player"`
	UnlockedCount int    `json:"UnlockedCount"`
	LockedCount   int    `json:"LockedCount"`
	TotalCount    int    `json:"TotalCount"`
}

type RESTTechs struct {
	Unlocked []string `json:"Unlocked"`
}

type RESTTechsResponse struct {
	Meta  RESTTechsMeta `json:"Meta"`
	Techs RESTTechs     `json:"Techs"`
}

type TechnologyRequest struct {
	Technology any `json:"Technology"`
}

type LearnTechnologyResponse struct {
	UnlockedCount int      `json:"UnlockedCount"`
	Unlocked      []string `json:"Unlocked"`
	Skipped       []string `json:"Skipped"`
}

type ForgetTechnologyResponse struct {
	ForgottenCount int      `json:"ForgottenCount"`
	Forgotten      any      `json:"Forgotten"`
	Skipped        []string `json:"Skipped"`
}

type RESTPalsMeta struct {
	PlayerUID     string `json:"PlayerUID"`
	Player        string `json:"Player"`
	TeamCount     int    `json:"TeamCount"`
	PalboxCount   int    `json:"PalboxCount"`
	BaseCampCount int    `json:"BaseCampCount"`
}

type RESTPalsResponse struct {
	Meta RESTPalsMeta   `json:"Meta"`
	Pals map[string]any `json:"Pals"`
}

type PalGrant struct {
	PalID string `json:"PalID"`
	Level int    `json:"Level"`
}

type GivePalsRequest struct {
	Pals []PalGrant `json:"Pals"`
}

type GivePalsResponse struct {
	Granted struct {
		Pals int `json:"Pals"`
	} `json:"Granted"`
}

type GivePalTemplatesRequest struct {
	PalTemplates []string `json:"PalTemplates"`
}

type GivePalTemplatesResponse struct {
	Granted struct {
		PalTemplates int `json:"PalTemplates"`
	} `json:"Granted"`
}

func (m Manager) RESTProgression(ctx context.Context, identifier string) (RESTProgressionResponse, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return RESTProgressionResponse{}, err
	}
	var response RESTProgressionResponse
	path := "/v1/pdapi/progression/" + url.PathEscape(identifier)
	if err := m.gmDoREST(ctx, http.MethodGet, path, nil, &response); err != nil {
		return RESTProgressionResponse{}, err
	}
	if strings.TrimSpace(response.Meta.Player) == "" && strings.TrimSpace(response.Meta.PlayerUID) == "" {
		return RESTProgressionResponse{}, invalidRESTResponse("progression payload did not contain a player identifier")
	}
	if response.Progression.Player.Level < 0 || response.Progression.Player.EXP < 0 || response.Progression.Player.UnusedStatusPoints < 0 ||
		response.Progression.Currencies.TechnologyPoints < 0 || response.Progression.Currencies.AncientTechnologyPoints < 0 {
		return RESTProgressionResponse{}, invalidRESTResponse("progression payload contained a negative value")
	}
	return response, nil
}

func (m Manager) RESTGiveProgression(ctx context.Context, identifier string, request GiveProgressionRequest) (GiveProgressionResponse, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return GiveProgressionResponse{}, err
	}
	if request.EXP == nil && request.TechnologyPoints == nil && request.AncientTechnologyPoints == nil && len(request.Relics) == 0 {
		return GiveProgressionResponse{}, invalidRESTRequest("at least one progression grant is required")
	}
	for name, value := range map[string]*int64{
		"EXP":                     request.EXP,
		"TechnologyPoints":        request.TechnologyPoints,
		"AncientTechnologyPoints": request.AncientTechnologyPoints,
	} {
		if value != nil && (*value <= 0 || *value > maxProgressionGrant) {
			return GiveProgressionResponse{}, invalidRESTRequest("%s must be between 1 and %d", name, maxProgressionGrant)
		}
	}
	for relicType, value := range request.Relics {
		if !progressionRelicTypes[relicType] {
			return GiveProgressionResponse{}, invalidRESTRequest("unsupported relic type %q", relicType)
		}
		if value <= 0 || value > maxProgressionGrant {
			return GiveProgressionResponse{}, invalidRESTRequest("relic %s must be between 1 and %d", relicType, maxProgressionGrant)
		}
	}
	var response GiveProgressionResponse
	path := "/v1/pdapi/give/progression/" + url.PathEscape(identifier)
	if err := m.gmDoREST(ctx, http.MethodPost, path, request, &response); err != nil {
		return GiveProgressionResponse{}, err
	}
	if response.Granted == nil {
		return GiveProgressionResponse{}, invalidRESTResponse("progression grant payload did not contain Granted")
	}
	return response, nil
}

func (m Manager) RESTTechs(ctx context.Context, identifier string) (RESTTechsResponse, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return RESTTechsResponse{}, err
	}
	var response RESTTechsResponse
	path := "/v1/pdapi/techs/" + url.PathEscape(identifier)
	if err := m.gmDoREST(ctx, http.MethodGet, path, nil, &response); err != nil {
		return RESTTechsResponse{}, err
	}
	if strings.TrimSpace(response.Meta.Player) == "" && strings.TrimSpace(response.Meta.PlayerUID) == "" {
		return RESTTechsResponse{}, invalidRESTResponse("technology payload did not contain a player identifier")
	}
	if response.Meta.UnlockedCount < 0 || response.Meta.LockedCount < 0 || response.Meta.TotalCount < 0 || response.Techs.Unlocked == nil {
		return RESTTechsResponse{}, invalidRESTResponse("technology payload contained invalid counts or no unlocked list")
	}
	return response, nil
}

func (m Manager) RESTLearnTechnology(ctx context.Context, identifier string, request TechnologyRequest) (LearnTechnologyResponse, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return LearnTechnologyResponse{}, err
	}
	selection, err := normalizeTechnologySelection(request.Technology)
	if err != nil {
		return LearnTechnologyResponse{}, err
	}
	request.Technology = selection
	var response LearnTechnologyResponse
	path := "/v1/pdapi/learntech/" + url.PathEscape(identifier)
	if err := m.gmDoREST(ctx, http.MethodPost, path, request, &response); err != nil {
		return LearnTechnologyResponse{}, err
	}
	if response.UnlockedCount < 0 || response.Unlocked == nil || response.Skipped == nil {
		return LearnTechnologyResponse{}, invalidRESTResponse("learn technology payload was incomplete")
	}
	return response, nil
}

func (m Manager) RESTForgetTechnology(ctx context.Context, identifier string, request TechnologyRequest) (ForgetTechnologyResponse, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return ForgetTechnologyResponse{}, err
	}
	selection, err := normalizeTechnologySelection(request.Technology)
	if err != nil {
		return ForgetTechnologyResponse{}, err
	}
	request.Technology = selection
	var response ForgetTechnologyResponse
	path := "/v1/pdapi/forgettech/" + url.PathEscape(identifier)
	if err := m.gmDoREST(ctx, http.MethodPost, path, request, &response); err != nil {
		return ForgetTechnologyResponse{}, err
	}
	if response.ForgottenCount < 0 || response.Forgotten == nil || response.Skipped == nil {
		return ForgetTechnologyResponse{}, invalidRESTResponse("forget technology payload was incomplete")
	}
	return response, nil
}

func normalizeTechnologySelection(value any) (any, error) {
	validate := func(raw string, allAllowed bool) (string, error) {
		raw = strings.TrimSpace(raw)
		if allAllowed && strings.EqualFold(raw, "all") {
			return "All", nil
		}
		if !itemIdentifierPattern.MatchString(raw) {
			return "", invalidRESTRequest("invalid technology identifier")
		}
		return raw, nil
	}
	switch selection := value.(type) {
	case string:
		return validate(selection, true)
	case []any:
		if len(selection) == 0 || len(selection) > maxTechnologySelections {
			return nil, invalidRESTRequest("Technology must contain between 1 and %d identifiers", maxTechnologySelections)
		}
		out := make([]string, 0, len(selection))
		seen := map[string]bool{}
		for _, item := range selection {
			raw, ok := item.(string)
			if !ok {
				return nil, invalidRESTRequest("Technology array must contain only strings")
			}
			normalized, err := validate(raw, false)
			if err != nil || strings.EqualFold(normalized, "all") {
				return nil, invalidRESTRequest("Technology array contains an invalid identifier")
			}
			key := strings.ToLower(normalized)
			if !seen[key] {
				seen[key] = true
				out = append(out, normalized)
			}
		}
		return out, nil
	case []string:
		items := make([]any, len(selection))
		for index := range selection {
			items[index] = selection[index]
		}
		return normalizeTechnologySelection(items)
	default:
		return nil, invalidRESTRequest("Technology must be a string or string array")
	}
}

func (m Manager) RESTPals(ctx context.Context, identifier string) (RESTPalsResponse, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return RESTPalsResponse{}, err
	}
	var response RESTPalsResponse
	path := "/v1/pdapi/pals/" + url.PathEscape(identifier)
	if err := m.gmDoREST(ctx, http.MethodGet, path, nil, &response); err != nil {
		return RESTPalsResponse{}, err
	}
	if strings.TrimSpace(response.Meta.Player) == "" && strings.TrimSpace(response.Meta.PlayerUID) == "" {
		return RESTPalsResponse{}, invalidRESTResponse("Pals payload did not contain a player identifier")
	}
	if response.Meta.TeamCount < 0 || response.Meta.PalboxCount < 0 || response.Meta.BaseCampCount < 0 || response.Pals == nil {
		return RESTPalsResponse{}, invalidRESTResponse("Pals payload contained invalid counts or no Pals object")
	}
	return response, nil
}

func (m Manager) RESTGivePals(ctx context.Context, identifier string, request GivePalsRequest) (GivePalsResponse, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return GivePalsResponse{}, err
	}
	if len(request.Pals) == 0 || len(request.Pals) > maxPalGrants {
		return GivePalsResponse{}, invalidRESTRequest("Pals must contain between 1 and %d grants", maxPalGrants)
	}
	for index := range request.Pals {
		request.Pals[index].PalID = strings.TrimSpace(request.Pals[index].PalID)
		if !itemIdentifierPattern.MatchString(request.Pals[index].PalID) {
			return GivePalsResponse{}, invalidRESTRequest("Pal %d has an invalid PalID", index+1)
		}
		if request.Pals[index].Level < 1 || request.Pals[index].Level > 255 {
			return GivePalsResponse{}, invalidRESTRequest("Pal %d Level must be between 1 and 255", index+1)
		}
	}
	var response GivePalsResponse
	path := "/v1/pdapi/give/pals/" + url.PathEscape(identifier)
	if err := m.gmDoREST(ctx, http.MethodPost, path, request, &response); err != nil {
		return GivePalsResponse{}, err
	}
	if response.Granted.Pals < 0 {
		return GivePalsResponse{}, invalidRESTResponse("Pal grant payload contained a negative result")
	}
	return response, nil
}

func (m Manager) RESTGivePalTemplates(ctx context.Context, identifier string, request GivePalTemplatesRequest) (GivePalTemplatesResponse, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return GivePalTemplatesResponse{}, err
	}
	if len(request.PalTemplates) == 0 || len(request.PalTemplates) > maxPalTemplateGrants {
		return GivePalTemplatesResponse{}, invalidRESTRequest("PalTemplates must contain between 1 and %d filenames", maxPalTemplateGrants)
	}
	for index := range request.PalTemplates {
		name, err := normalizePalTemplateName(request.PalTemplates[index])
		if err != nil {
			return GivePalTemplatesResponse{}, err
		}
		request.PalTemplates[index] = name
	}
	var response GivePalTemplatesResponse
	path := "/v1/pdapi/give/paltemplate/" + url.PathEscape(identifier)
	if err := m.gmDoREST(ctx, http.MethodPost, path, request, &response); err != nil {
		return GivePalTemplatesResponse{}, err
	}
	if response.Granted.PalTemplates < 0 {
		return GivePalTemplatesResponse{}, invalidRESTResponse("Pal template grant payload contained a negative result")
	}
	return response, nil
}
