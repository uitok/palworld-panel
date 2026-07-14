package mods

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"palpanel/internal/aitranslation"
	"palpanel/internal/appconfig"
)

const (
	steamWorkshopPageURL   = "https://steamcommunity.com/sharedfiles/filedetails/?id="
	workshopCacheTTL       = 90 * time.Second
	workshopRequestTimeout = time.Duration(appconfig.DefaultSteamAPITimeoutSeconds) * time.Second
)

var ErrSteamAPIKeyMissing = errors.New("Steam Web API key is not available")

type SteamAPIError struct {
	Status  int
	Code    string
	Message string
}

func (e SteamAPIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Code != "" {
		return e.Code
	}
	return "steam api request failed"
}

type WorkshopSearchParams struct {
	Query    string
	Sort     string
	Cursor   string
	PageSize int
	Tags     []string
}

type WorkshopSearchResult struct {
	Items      []WorkshopItem `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
	Total      int            `json:"total"`
	PageSize   int            `json:"page_size"`
}

type WorkshopItem struct {
	ID              string                     `json:"id"`
	Title           string                     `json:"title"`
	Summary         string                     `json:"summary,omitempty"`
	PreviewURL      string                     `json:"preview_url,omitempty"`
	SteamURL        string                     `json:"steam_url"`
	Tags            []string                   `json:"tags,omitempty"`
	FileSize        int64                      `json:"file_size,omitempty"`
	Subscriptions   int64                      `json:"subscriptions,omitempty"`
	TimeCreated     int64                      `json:"time_created,omitempty"`
	TimeUpdated     int64                      `json:"time_updated,omitempty"`
	Installed       bool                       `json:"installed"`
	Enabled         bool                       `json:"enabled"`
	UpdateAvailable bool                       `json:"update_available"`
	ModID           string                     `json:"mod_id,omitempty"`
	Translation     *aitranslation.Translation `json:"translation,omitempty"`
}

type SteamClient struct {
	apiKey     string
	appID      string
	baseURL    string
	httpClient *http.Client
}

func NewSteamClient(apiKey, appID string) *SteamClient {
	return &SteamClient{
		apiKey:  strings.TrimSpace(apiKey),
		appID:   strings.TrimSpace(appID),
		baseURL: appconfig.DefaultSteamAPIBaseURL,
		httpClient: &http.Client{
			Timeout: workshopRequestTimeout,
		},
	}
}

func (c *SteamClient) QueryFiles(ctx context.Context, params WorkshopSearchParams) (WorkshopSearchResult, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return WorkshopSearchResult{}, ErrSteamAPIKeyMissing
	}
	pageSize := normalizePageSize(params.PageSize)
	cursor := strings.TrimSpace(params.Cursor)
	if cursor == "" {
		cursor = "*"
	}

	values := url.Values{}
	values.Set("key", c.apiKey)
	values.Set("format", "json")
	values.Set("query_type", queryTypeForSort(params.Sort))
	values.Set("page", "1")
	values.Set("cursor", cursor)
	values.Set("numperpage", strconv.Itoa(pageSize))
	values.Set("creator_appid", c.appID)
	values.Set("appid", c.appID)
	values.Set("filetype", "0")
	values.Set("return_tags", "1")
	values.Set("return_previews", "1")
	values.Set("return_short_description", "1")
	values.Set("return_metadata", "1")
	values.Set("cache_max_age_seconds", "60")
	if q := strings.TrimSpace(params.Query); q != "" {
		values.Set("search_text", q)
	}
	if len(params.Tags) > 0 {
		values.Set("match_all_tags", "false")
		for i, tag := range params.Tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				values.Set(fmt.Sprintf("requiredtags[%d]", i), tag)
			}
		}
	}

	var payload queryFilesResponse
	if err := c.doJSON(ctx, http.MethodGet, "/IPublishedFileService/QueryFiles/v1/", values, nil, &payload); err != nil {
		return WorkshopSearchResult{}, err
	}
	if payload.Response.Result != 0 && payload.Response.Result != 1 {
		return WorkshopSearchResult{}, SteamAPIError{Code: "steam_query_failed", Message: fmt.Sprintf("Steam QueryFiles returned result %d", payload.Response.Result)}
	}
	return WorkshopSearchResult{
		Items:      mapSteamItems(payload.Response.Details),
		NextCursor: payload.Response.NextCursor,
		Total:      payload.Response.Total,
		PageSize:   pageSize,
	}, nil
}

func (c *SteamClient) GetPublishedFileDetails(ctx context.Context, ids []string) ([]WorkshopItem, error) {
	return c.GetDetails(ctx, ids)
}

func (c *SteamClient) GetDetails(ctx context.Context, ids []string) ([]WorkshopItem, error) {
	cleanIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			cleanIDs = append(cleanIDs, id)
		}
	}
	if len(cleanIDs) == 0 {
		return nil, nil
	}
	values := url.Values{}
	values.Set("format", "json")
	values.Set("itemcount", strconv.Itoa(len(cleanIDs)))
	for i, id := range cleanIDs {
		values.Set(fmt.Sprintf("publishedfileids[%d]", i), id)
	}

	var payload publishedFileDetailsResponse
	if err := c.doJSON(ctx, http.MethodPost, "/ISteamRemoteStorage/GetPublishedFileDetails/v1/", nil, values, &payload); err != nil {
		return nil, err
	}
	if payload.Response.Result != 0 && payload.Response.Result != 1 {
		return nil, SteamAPIError{Code: "steam_details_failed", Message: fmt.Sprintf("Steam GetDetails returned result %d", payload.Response.Result)}
	}
	items := mapSteamDetailItems(payload.Response.Details)
	return items, nil
}

func (c *SteamClient) doJSON(ctx context.Context, method, path string, query url.Values, form url.Values, out any) error {
	endpoint := strings.TrimRight(c.baseURL, "/") + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	var body io.Reader
	if len(form) > 0 {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return SteamAPIError{Code: "steam_request_invalid", Message: "Steam API request could not be created"}
	}
	if len(form) > 0 {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return SteamAPIError{Code: "steam_timeout", Message: "Steam API request timed out"}
		}
		if errors.As(err, &netErr) && netErr.Timeout() {
			return SteamAPIError{Code: "steam_timeout", Message: "Steam API request timed out"}
		}
		return SteamAPIError{Code: "steam_unreachable", Message: "Steam API is unreachable"}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		return SteamAPIError{Status: resp.StatusCode, Code: "steam_http_error", Message: fmt.Sprintf("Steam API returned HTTP %d", resp.StatusCode)}
	}

	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	if err := decoder.Decode(out); err != nil {
		return SteamAPIError{Code: "steam_decode_failed", Message: err.Error()}
	}
	return nil
}

type WorkshopService struct {
	client         *SteamClient
	requestTimeout time.Duration
	mu             sync.Mutex
	cache          map[string]cachedWorkshopSearch
}

type cachedWorkshopSearch struct {
	expiresAt time.Time
	result    WorkshopSearchResult
}

func NewWorkshopService(cfg appconfig.Config) *WorkshopService {
	timeout := time.Duration(cfg.SteamAPITimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = workshopRequestTimeout
	}
	client := NewSteamClient(cfg.EffectiveSteamWebAPIKey(), cfg.WorkshopAppID)
	if baseURL := strings.TrimRight(strings.TrimSpace(cfg.SteamAPIBaseURL), "/"); baseURL != "" {
		client.baseURL = baseURL
	}
	client.httpClient.Timeout = timeout
	return &WorkshopService{
		client:         client,
		requestTimeout: timeout,
		cache:          map[string]cachedWorkshopSearch{},
	}
}

func (s *WorkshopService) Search(ctx context.Context, params WorkshopSearchParams) (WorkshopSearchResult, error) {
	key := searchCacheKey(params)
	now := time.Now()
	s.mu.Lock()
	if cached, ok := s.cache[key]; ok && now.Before(cached.expiresAt) {
		result := cached.result
		s.mu.Unlock()
		return result, nil
	}
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()
	result, err := s.client.QueryFiles(ctx, params)
	if err != nil {
		return WorkshopSearchResult{}, err
	}
	s.mu.Lock()
	s.cache[key] = cachedWorkshopSearch{expiresAt: now.Add(workshopCacheTTL), result: result}
	s.mu.Unlock()
	return result, nil
}

func (s *WorkshopService) Detail(ctx context.Context, itemID string) (WorkshopItem, error) {
	ctx, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()
	items, err := s.client.GetDetails(ctx, []string{itemID})
	if err != nil {
		return WorkshopItem{}, err
	}
	if len(items) == 0 {
		return WorkshopItem{}, fmt.Errorf("Steam Workshop item not found")
	}
	return items[0], nil
}

func searchCacheKey(params WorkshopSearchParams) string {
	pageSize := normalizePageSize(params.PageSize)
	tags := normalizeTags(params.Tags)
	return strings.Join([]string{
		strings.TrimSpace(params.Query),
		strings.TrimSpace(params.Sort),
		strings.TrimSpace(params.Cursor),
		strconv.Itoa(pageSize),
		strings.Join(tags, ","),
	}, "\x00")
}

func queryTypeForSort(sort string) string {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "trend":
		return "3"
	case "new":
		return "1"
	case "updated":
		return "21"
	case "popular", "":
		return "12"
	default:
		return "12"
	}
}

func normalizePageSize(pageSize int) int {
	if pageSize <= 0 {
		return 24
	}
	if pageSize > 50 {
		return 50
	}
	return pageSize
}

func normalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[strings.ToLower(tag)] {
			continue
		}
		seen[strings.ToLower(tag)] = true
		out = append(out, tag)
	}
	return out
}

type queryFilesResponse struct {
	Response struct {
		Result     int               `json:"result"`
		Total      int               `json:"total"`
		NextCursor string            `json:"next_cursor"`
		Details    []steamFileDetail `json:"publishedfiledetails"`
	} `json:"response"`
}

type publishedFileDetailsResponse struct {
	Response struct {
		Result  int               `json:"result"`
		Details []steamFileDetail `json:"publishedfiledetails"`
	} `json:"response"`
}

type steamFileDetail struct {
	PublishedFileID       string      `json:"publishedfileid"`
	Result                int         `json:"result"`
	Title                 string      `json:"title"`
	FileDescription       string      `json:"file_description"`
	Description           string      `json:"description"`
	ShortDescription      string      `json:"short_description"`
	PreviewURL            string      `json:"preview_url"`
	FileSize              json.Number `json:"file_size"`
	Subscriptions         json.Number `json:"subscriptions"`
	LifetimeSubscriptions json.Number `json:"lifetime_subscriptions"`
	TimeCreated           json.Number `json:"time_created"`
	TimeUpdated           json.Number `json:"time_updated"`
	Tags                  []struct {
		Tag string `json:"tag"`
	} `json:"tags"`
}

func mapSteamItems(details []steamFileDetail) []WorkshopItem {
	return mapSteamItemsWithSummary(details, false)
}

func mapSteamDetailItems(details []steamFileDetail) []WorkshopItem {
	return mapSteamItemsWithSummary(details, true)
}

func mapSteamItemsWithSummary(details []steamFileDetail, fullDescription bool) []WorkshopItem {
	items := make([]WorkshopItem, 0, len(details))
	for _, detail := range details {
		if detail.Result != 0 && detail.Result != 1 {
			continue
		}
		id := strings.TrimSpace(detail.PublishedFileID)
		if id == "" {
			continue
		}
		subscriptions := numberInt64(detail.Subscriptions)
		if subscriptions == 0 {
			subscriptions = numberInt64(detail.LifetimeSubscriptions)
		}
		item := WorkshopItem{
			ID:            id,
			Title:         strings.TrimSpace(detail.Title),
			Summary:       summaryFor(detail, fullDescription),
			PreviewURL:    strings.TrimSpace(detail.PreviewURL),
			SteamURL:      steamWorkshopPageURL + id,
			Tags:          tagsFor(detail),
			FileSize:      numberInt64(detail.FileSize),
			Subscriptions: subscriptions,
			TimeCreated:   numberInt64(detail.TimeCreated),
			TimeUpdated:   numberInt64(detail.TimeUpdated),
		}
		if item.Title == "" {
			item.Title = id
		}
		items = append(items, item)
	}
	return items
}

func summaryFor(detail steamFileDetail, fullDescription bool) string {
	if fullDescription {
		if text := strings.TrimSpace(detail.FileDescription); text != "" {
			return text
		}
		if text := strings.TrimSpace(detail.Description); text != "" {
			return text
		}
	}
	if text := strings.TrimSpace(detail.ShortDescription); text != "" {
		return text
	}
	text := strings.TrimSpace(detail.FileDescription)
	if text == "" {
		text = strings.TrimSpace(detail.Description)
	}
	const limit = 500
	if len(text) > limit {
		return strings.TrimSpace(text[:limit]) + "..."
	}
	return text
}

func tagsFor(detail steamFileDetail) []string {
	out := make([]string, 0, len(detail.Tags))
	for _, tag := range detail.Tags {
		value := strings.TrimSpace(tag.Tag)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func numberInt64(n json.Number) int64 {
	if n == "" {
		return 0
	}
	if i, err := n.Int64(); err == nil {
		return i
	}
	f, err := strconv.ParseFloat(n.String(), 64)
	if err != nil {
		return 0
	}
	return int64(f)
}
