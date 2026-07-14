package paldefender

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	restResponseLimit = 8 << 20
	maxItemGrants     = 100
)

var (
	ErrRESTTokenMissing     = errors.New("PalDefender REST token has not been generated")
	ErrRESTResponseTooLarge = errors.New("PalDefender REST response is too large")
	ErrInvalidRESTRequest   = errors.New("invalid PalDefender REST request")
	playerIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
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
	Configured bool         `json:"configured"`
	Available  bool         `json:"available"`
	Version    *RESTVersion `json:"version,omitempty"`
	Error      string       `json:"error,omitempty"`
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
		Timeout:   10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			// Never forward the PalDefender bearer token to a redirect target.
			return http.ErrUseLastResponse
		},
	}
}

func (m Manager) GMStatus(ctx context.Context) GMStatus {
	status := GMStatus{}
	token, found, err := m.store.GetKV(ctx, kvRESTToken)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Configured = found && strings.TrimSpace(token) != ""
	if !status.Configured {
		return status
	}
	version, err := m.RESTVersion(ctx)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Available = true
	status.Version = &version
	return status
}

func (m Manager) RESTVersion(ctx context.Context) (RESTVersion, error) {
	var response struct {
		Version RESTVersion `json:"Version"`
	}
	if err := m.doREST(ctx, http.MethodGet, "/v1/pdapi/version", nil, &response); err != nil {
		return RESTVersion{}, err
	}
	return response.Version, nil
}

func (m Manager) RESTPlayers(ctx context.Context) (RESTPlayersResponse, error) {
	var response RESTPlayersResponse
	if err := m.doREST(ctx, http.MethodGet, "/v1/pdapi/players", nil, &response); err != nil {
		return RESTPlayersResponse{}, err
	}
	if response.Players == nil {
		response.Players = []RESTPlayer{}
	}
	return response, nil
}

func (m Manager) RESTInventory(ctx context.Context, identifier string) (RESTInventoryResponse, error) {
	identifier, err := validatePlayerIdentifier(identifier)
	if err != nil {
		return RESTInventoryResponse{}, err
	}
	var response RESTInventoryResponse
	path := "/v1/pdapi/items/" + url.PathEscape(identifier)
	if err := m.doREST(ctx, http.MethodGet, path, nil, &response); err != nil {
		return RESTInventoryResponse{}, err
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
		if request.Items[index].ItemID == "" || len(request.Items[index].ItemID) > 128 || strings.ContainsAny(request.Items[index].ItemID, "\r\n\x00") {
			return GiveItemsResponse{}, invalidRESTRequest("item %d has an invalid ItemID", index+1)
		}
		if request.Items[index].Count <= 0 || request.Items[index].Count > 2_147_483_647 {
			return GiveItemsResponse{}, invalidRESTRequest("item %d Count must be between 1 and 2147483647", index+1)
		}
	}
	var response GiveItemsResponse
	path := "/v1/pdapi/give/items/" + url.PathEscape(identifier)
	if err := m.doREST(ctx, http.MethodPost, path, request, &response); err != nil {
		return GiveItemsResponse{}, err
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
	if err := m.doREST(ctx, http.MethodPost, "/v1/pdapi/SendPlayerMessage", request, &response); err != nil {
		return nil, err
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
	if err := m.doREST(ctx, http.MethodPost, path, map[string]string{"Message": message}, &response); err != nil {
		return nil, err
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
	if err := m.doREST(ctx, http.MethodPost, path, request, &response); err != nil {
		return nil, err
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

func (m Manager) doREST(ctx context.Context, method, path string, body, out any) error {
	token, found, err := m.store.GetKV(ctx, kvRESTToken)
	if err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if !found || token == "" {
		return ErrRESTTokenMissing
	}

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(m.restBaseURL, "/")+path, reader)
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
		return fmt.Errorf("PalDefender REST API request failed: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, restResponseLimit+1))
	if err != nil {
		return err
	}
	if len(payload) > restResponseLimit {
		return ErrRESTResponseTooLarge
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return decodeRESTError(resp.StatusCode, payload)
	}
	if out == nil || len(bytes.TrimSpace(payload)) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode PalDefender REST response: %w", err)
	}
	return nil
}

func decodeRESTError(status int, payload []byte) error {
	var envelope struct {
		Error struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
	}
	_ = json.Unmarshal(payload, &envelope)
	message := strings.TrimSpace(envelope.Error.Message)
	if message == "" {
		message = http.StatusText(status)
	}
	return &RESTError{Status: status, Code: strings.TrimSpace(envelope.Error.Code), Message: message}
}
