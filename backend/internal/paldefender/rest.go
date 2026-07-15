package paldefender

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	restResponseLimit  = 8 << 20
	maxItemGrants      = 100
	maxRESTErrorRunes  = 2048
	restRequestTimeout = 5 * time.Second
)

var (
	ErrRESTTokenMissing         = errors.New("PalDefender REST token has not been generated")
	ErrRESTResponseTooLarge     = errors.New("PalDefender REST response is too large")
	ErrRESTInvalidResponse      = errors.New("PalDefender REST returned an invalid response")
	ErrRESTTimeout              = errors.New("PalDefender REST request timed out")
	ErrRESTUnavailable          = errors.New("PalDefender REST is unavailable")
	ErrRESTInvalidConfiguration = errors.New("PalDefender REST endpoint must be an HTTP loopback address")
	ErrPalDefenderNotInstalled  = errors.New("PalDefender is not installed")
	ErrPalDefenderNotLoaded     = errors.New("PalDefender startup loading has not been verified")
	ErrPalDefenderRESTDisabled  = errors.New("PalDefender REST API is disabled")
	ErrInvalidRESTRequest       = errors.New("invalid PalDefender REST request")
	playerIdentifierPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	itemIdentifierPattern       = regexp.MustCompile(`^[A-Za-z0-9_:-]{1,128}$`)
)

type GMState string

const (
	GMStateReady            GMState = "ready"
	GMStateNotInstalled     GMState = "not_installed"
	GMStateNotLoaded        GMState = "not_loaded"
	GMStateNotConfigured    GMState = "not_configured"
	GMStateRESTDisabled     GMState = "rest_disabled"
	GMStateServerNotRunning GMState = "server_not_running"
	GMStateFailed           GMState = "failed"
)

type RESTError struct {
	Status  int
	Code    string
	Message string
}

func (e *RESTError) Error() string {
	if e.Code != "" && e.Message != "" {
		return fmt.Sprintf("PalDefender REST API returned %s: %s", e.Code, e.Message)
	}
	if e.Message != "" {
		return "PalDefender REST API: " + e.Message
	}
	return fmt.Sprintf("PalDefender REST API returned HTTP %d", e.Status)
}

type RESTVersion struct {
	Major       int    `json:"Major"`
	Minor       int    `json:"Minor"`
	Patch       int    `json:"Patch"`
	Build       int    `json:"Build"`
	Version     string `json:"Version"`
	VersionLong string `json:"VersionLong"`
	Beta        bool   `json:"Beta"`
}

type GMStatus struct {
	Configured   bool         `json:"configured"`
	Available    bool         `json:"available"`
	Installed    bool         `json:"installed"`
	LoadVerified bool         `json:"load_verified"`
	RESTEnabled  bool         `json:"rest_enabled"`
	State        GMState      `json:"state"`
	Version      *RESTVersion `json:"version,omitempty"`
	Error        string       `json:"error,omitempty"`
}

type RESTLocation struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type RESTPlayer struct {
	Name          string       `json:"Name"`
	IP            string       `json:"IP"`
	PlayerUID     string       `json:"PlayerUID"`
	UserID        string       `json:"UserId"`
	GuildName     string       `json:"GuildName"`
	GuildUUID     string       `json:"GuildUUID"`
	Status        string       `json:"Status"`
	WorldLocation RESTLocation `json:"WorldLocation"`
	MapLocation   RESTLocation `json:"MapLocation"`
}

type RESTPlayersMeta struct {
	PlayerCount int `json:"PlayerCount"`
	OnlineCount int `json:"OnlineCount"`
}

type RESTPlayersResponse struct {
	Meta    RESTPlayersMeta `json:"Meta"`
	Players []RESTPlayer    `json:"Players"`
}

type RESTInventoryMeta struct {
	PlayerUID string `json:"PlayerUID"`
	Player    string `json:"Player"`
}

type RESTInventorySlot struct {
	ItemID string `json:"ItemID"`
	Count  int64  `json:"Count"`
}

type RESTInventoryContainer struct {
	Available   bool                         `json:"Available"`
	ContainerID string                       `json:"ContainerID"`
	UsedSlots   int                          `json:"UsedSlots"`
	MaxSlots    int                          `json:"MaxSlots"`
	FreeSlots   int                          `json:"FreeSlots"`
	Slots       map[string]RESTInventorySlot `json:"Slots"`
}

type RESTInventory struct {
	Items    RESTInventoryContainer `json:"Items"`
	KeyItems RESTInventoryContainer `json:"KeyItems"`
	Weapons  RESTInventoryContainer `json:"Weapons"`
	Armor    RESTInventoryContainer `json:"Armor"`
	Food     RESTInventoryContainer `json:"Food"`
	DropSlot RESTInventoryContainer `json:"DropSlot"`
}

type RESTInventoryResponse struct {
	Meta      RESTInventoryMeta `json:"Meta"`
	Inventory RESTInventory     `json:"Inventory"`
}

type ItemGrant struct {
	ItemID string `json:"ItemID"`
	Count  int64  `json:"Count"`
}

type GiveItemsRequest struct {
	Items []ItemGrant `json:"Items"`
}

type GiveItemsResponse struct {
	Granted struct {
		Items int64 `json:"Items"`
	} `json:"Granted"`
}

type SendMessageRequest struct {
	SendType string `json:"SendType"`
	Message  string `json:"Message"`
	UserID   string `json:"UserID"`
}

type PunishmentRequest struct {
	Reason string `json:"Reason,omitempty"`
	IP     bool   `json:"IP,omitempty"`
}

type RESTActionResult map[string]any

func newRESTHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{
		Transport: transport,
		Timeout:   restRequestTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			// Never forward the PalDefender bearer token to a redirect target.
			return http.ErrUseLastResponse
		},
	}
}

func (m Manager) GMStatus(ctx context.Context) GMStatus {
	status := GMStatus{State: GMStateFailed}
	local, err := m.Status(ctx)
	if err != nil {
		status.Error = "could not inspect PalDefender installation"
		return status
	}
	status.Installed = local.Installed
	status.LoadVerified = local.LoadVerified
	status.RESTEnabled = local.RESTAPIEnabled
	token, found, err := m.store.GetKV(ctx, kvRESTToken)
	if err != nil {
		status.Error = "could not read PalDefender REST configuration"
		return status
	}
	status.Configured = found && strings.TrimSpace(token) != ""
	if !status.Installed {
		status.State = GMStateNotInstalled
		return status
	}
	if !status.LoadVerified {
		status.State = GMStateNotLoaded
		return status
	}
	if !status.Configured {
		status.State = GMStateNotConfigured
		return status
	}
	if !status.RESTEnabled {
		status.State = GMStateRESTDisabled
		return status
	}
	version, err := m.RESTVersion(ctx)
	if err != nil {
		if errors.Is(err, ErrRESTUnavailable) {
			status.State = GMStateServerNotRunning
		} else {
			status.State = GMStateFailed
		}
		status.Error = safeRESTStatusError(err)
		return status
	}
	status.Available = true
	status.State = GMStateReady
	status.Version = &version
	return status
}

func (m Manager) RESTVersion(ctx context.Context) (RESTVersion, error) {
	var response struct {
		Version RESTVersion `json:"Version"`
	}
	if err := m.gmDoREST(ctx, http.MethodGet, "/v1/pdapi/version", nil, &response); err != nil {
		return RESTVersion{}, err
	}
	if strings.TrimSpace(response.Version.Version) == "" && strings.TrimSpace(response.Version.VersionLong) == "" {
		return RESTVersion{}, invalidRESTResponse("version payload did not contain a version string")
	}
	return response.Version, nil
}

func (m Manager) RESTPlayers(ctx context.Context) (RESTPlayersResponse, error) {
	var response RESTPlayersResponse
	if err := m.gmDoREST(ctx, http.MethodGet, "/v1/pdapi/players", nil, &response); err != nil {
		return RESTPlayersResponse{}, err
	}
	if response.Players == nil {
		return RESTPlayersResponse{}, invalidRESTResponse("players payload did not contain a player array")
	}
	if response.Meta.PlayerCount < 0 || response.Meta.OnlineCount < 0 || response.Meta.OnlineCount > response.Meta.PlayerCount {
		return RESTPlayersResponse{}, invalidRESTResponse("players payload contained invalid counts")
	}
	for _, player := range response.Players {
		if strings.TrimSpace(player.UserID) == "" && strings.TrimSpace(player.PlayerUID) == "" {
			return RESTPlayersResponse{}, invalidRESTResponse("players payload contained a player without an identifier")
		}
	}
	return response, nil
}

func (m Manager) RESTPlayer(ctx context.Context, identifier string) (RESTPlayer, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return RESTPlayer{}, err
	}
	response, err := m.RESTPlayers(ctx)
	if err != nil {
		return RESTPlayer{}, err
	}
	for _, player := range response.Players {
		if player.UserID == identifier || player.PlayerUID == identifier {
			return player, nil
		}
	}
	return RESTPlayer{}, &RESTError{Status: http.StatusNotFound, Code: "PLAYER_NOT_FOUND", Message: "player was not found"}
}

func (m Manager) RESTInventory(ctx context.Context, identifier string) (RESTInventoryResponse, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return RESTInventoryResponse{}, err
	}
	var response RESTInventoryResponse
	path := "/v1/pdapi/items/" + url.PathEscape(identifier)
	if err := m.gmDoREST(ctx, http.MethodGet, path, nil, &response); err != nil {
		return RESTInventoryResponse{}, err
	}
	if strings.TrimSpace(response.Meta.Player) == "" && strings.TrimSpace(response.Meta.PlayerUID) == "" {
		return RESTInventoryResponse{}, invalidRESTResponse("inventory payload did not contain a player identifier")
	}
	return response, nil
}

func (m Manager) RESTGiveItems(ctx context.Context, identifier string, request GiveItemsRequest) (GiveItemsResponse, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return GiveItemsResponse{}, err
	}
	if len(request.Items) == 0 || len(request.Items) > maxItemGrants {
		return GiveItemsResponse{}, invalidRESTRequest("items must contain between 1 and %d grants", maxItemGrants)
	}
	for index := range request.Items {
		request.Items[index].ItemID = strings.TrimSpace(request.Items[index].ItemID)
		if !itemIdentifierPattern.MatchString(request.Items[index].ItemID) {
			return GiveItemsResponse{}, invalidRESTRequest("item %d has an invalid ItemID", index+1)
		}
		if request.Items[index].Count <= 0 || request.Items[index].Count > 2_147_483_647 {
			return GiveItemsResponse{}, invalidRESTRequest("item %d Count must be between 1 and 2147483647", index+1)
		}
	}
	var response GiveItemsResponse
	path := "/v1/pdapi/give/items/" + url.PathEscape(identifier)
	if err := m.gmDoREST(ctx, http.MethodPost, path, request, &response); err != nil {
		return GiveItemsResponse{}, err
	}
	if response.Granted.Items < 0 {
		return GiveItemsResponse{}, invalidRESTResponse("item grant payload contained a negative result")
	}
	return response, nil
}

func (m Manager) RESTSendMessage(ctx context.Context, identifier string, request SendMessageRequest) (RESTActionResult, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return nil, err
	}
	request.UserID = identifier
	request.Message = strings.TrimSpace(request.Message)
	if request.Message == "" || len(request.Message) > 4096 {
		return nil, invalidRESTRequest("message must contain between 1 and 4096 bytes")
	}
	request.SendType = strings.TrimSpace(request.SendType)
	if request.SendType == "" {
		request.SendType = "PlayerLogImportant"
	}
	allowed := map[string]bool{
		"PlayerChat": true, "PlayerGlobalChat": true, "PlayerGuildChat": true,
		"PlayerLogNormal": true, "PlayerLogImportant": true, "PlayerLogVeryImportant": true,
	}
	if !allowed[request.SendType] {
		return nil, invalidRESTRequest("unsupported SendType %q", request.SendType)
	}
	var response RESTActionResult
	if err := m.gmDoREST(ctx, http.MethodPost, "/v1/pdapi/SendPlayerMessage", request, &response); err != nil {
		return nil, err
	}
	if response == nil {
		return nil, invalidRESTResponse("message payload was not an object")
	}
	return response, nil
}

func (m Manager) RESTBroadcast(ctx context.Context, message string, alert bool) (RESTActionResult, error) {
	message = strings.TrimSpace(message)
	if message == "" || len(message) > 4096 {
		return nil, invalidRESTRequest("message must contain between 1 and 4096 bytes")
	}
	path := "/v1/pdapi/Broadcast"
	if alert {
		path = "/v1/pdapi/Alert"
	}
	var response RESTActionResult
	if err := m.gmDoREST(ctx, http.MethodPost, path, map[string]string{"Message": message}, &response); err != nil {
		return nil, err
	}
	if response == nil {
		return nil, invalidRESTResponse("broadcast payload was not an object")
	}
	return response, nil
}

func (m Manager) RESTKick(ctx context.Context, identifier string, request PunishmentRequest) (RESTActionResult, error) {
	return m.restPunishment(ctx, "kick", identifier, request)
}

func (m Manager) RESTBan(ctx context.Context, identifier string, request PunishmentRequest) (RESTActionResult, error) {
	return m.restPunishment(ctx, "ban", identifier, request)
}

func (m Manager) RESTUnban(ctx context.Context, identifier string, request PunishmentRequest) (RESTActionResult, error) {
	request.IP = false
	return m.restPunishment(ctx, "unban", identifier, request)
}

func (m Manager) restPunishment(ctx context.Context, action, identifier string, request PunishmentRequest) (RESTActionResult, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return nil, err
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if len(request.Reason) > 1024 || strings.ContainsRune(request.Reason, '\x00') {
		return nil, invalidRESTRequest("reason must not exceed 1024 bytes")
	}
	var response RESTActionResult
	path := "/v1/pdapi/" + action + "/" + url.PathEscape(identifier)
	if err := m.gmDoREST(ctx, http.MethodPost, path, request, &response); err != nil {
		return nil, err
	}
	if response == nil {
		return nil, invalidRESTResponse("punishment payload was not an object")
	}
	return response, nil
}

func validatePlayerIdentifier(identifier string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	if !playerIdentifierPattern.MatchString(identifier) {
		return "", invalidRESTRequest("invalid player identifier")
	}
	return identifier, nil
}

func invalidRESTRequest(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRESTRequest, fmt.Sprintf(format, args...))
}

func invalidRESTResponse(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrRESTInvalidResponse, fmt.Sprintf(format, args...))
}

func (m Manager) gmDoREST(ctx context.Context, method, path string, body, out any) error {
	if err := m.validateGMPrerequisites(ctx); err != nil {
		return err
	}
	return m.doREST(ctx, method, path, body, out)
}

func (m Manager) doREST(ctx context.Context, method, path string, body, out any) error {
	token, found, err := m.store.GetKV(ctx, kvRESTToken)
	if err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if !found || token == "" {
		return ErrRESTTokenMissing
	}
	restURL, err := buildRESTURL(m.restBaseURL, path)
	if err != nil {
		return err
	}

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, restURL, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Origin", panelRESTOrigin)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := m.restClient
	if client == nil {
		client = newRESTHTTPClient()
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return context.Canceled
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("%w: %v", ErrRESTTimeout, err)
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return fmt.Errorf("%w: %v", ErrRESTTimeout, err)
		}
		return fmt.Errorf("%w: %v", ErrRESTUnavailable, err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, restResponseLimit+1))
	if err != nil {
		return fmt.Errorf("%w: response body could not be read", ErrRESTInvalidResponse)
	}
	if len(payload) > restResponseLimit {
		return ErrRESTResponseTooLarge
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return decodeRESTError(resp.StatusCode, payload)
	}
	if out == nil {
		return nil
	}
	if len(bytes.TrimSpace(payload)) == 0 {
		return fmt.Errorf("%w: response body was empty", ErrRESTInvalidResponse)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("%w: JSON could not be decoded", ErrRESTInvalidResponse)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: response contained trailing data", ErrRESTInvalidResponse)
	}
	return nil
}

func (m Manager) validateGMPrerequisites(ctx context.Context) error {
	status, err := m.Status(ctx)
	if err != nil {
		return err
	}
	if !status.Installed {
		return ErrPalDefenderNotInstalled
	}
	if !status.LoadVerified {
		return ErrPalDefenderNotLoaded
	}
	if !status.RESTAPIEnabled {
		return ErrPalDefenderRESTDisabled
	}
	return nil
}

func buildRESTURL(baseURL, path string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrRESTInvalidConfiguration
	}
	host := strings.TrimSpace(parsed.Hostname())
	loopback := strings.EqualFold(host, "localhost")
	if ip := net.ParseIP(host); ip != nil {
		loopback = ip.IsLoopback()
	}
	if !loopback || (parsed.Path != "" && parsed.Path != "/") {
		return "", ErrRESTInvalidConfiguration
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawPath = ""
	return parsed.String(), nil
}

func decodeRESTError(status int, payload []byte) error {
	var envelope struct {
		Error struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
	}
	_ = json.Unmarshal(payload, &envelope)
	message := sanitizeRESTErrorText(envelope.Error.Message)
	if message == "" {
		message = http.StatusText(status)
	}
	return &RESTError{Status: status, Code: sanitizeRESTErrorText(envelope.Error.Code), Message: message}
}

func sanitizeRESTErrorText(value string) string {
	value = strings.Map(func(char rune) rune {
		if char < 0x20 || char == 0x7f {
			return ' '
		}
		return char
	}, strings.TrimSpace(value))
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > maxRESTErrorRunes {
		value = string(runes[:maxRESTErrorRunes])
	}
	return value
}

func safeRESTStatusError(err error) string {
	var restErr *RESTError
	if errors.As(err, &restErr) {
		return restErr.Message
	}
	switch {
	case errors.Is(err, ErrRESTTimeout):
		return ErrRESTTimeout.Error()
	case errors.Is(err, ErrRESTInvalidResponse), errors.Is(err, ErrRESTResponseTooLarge):
		return ErrRESTInvalidResponse.Error()
	case errors.Is(err, ErrRESTInvalidConfiguration):
		return ErrRESTInvalidConfiguration.Error()
	case errors.Is(err, ErrPalDefenderNotInstalled):
		return ErrPalDefenderNotInstalled.Error()
	case errors.Is(err, ErrPalDefenderNotLoaded):
		return ErrPalDefenderNotLoaded.Error()
	case errors.Is(err, ErrPalDefenderRESTDisabled):
		return ErrPalDefenderRESTDisabled.Error()
	default:
		return ErrRESTUnavailable.Error()
	}
}
