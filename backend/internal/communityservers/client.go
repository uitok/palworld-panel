package communityservers

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
	"time"

	"golang.org/x/net/proxy"
)

const maximumResponseBytes = 4 << 20

var ErrResponseTooLarge = errors.New("community server response exceeds configured limit")

type Fetcher interface {
	Fetch(context.Context, Query) ([]Server, int, error)
}

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL, proxyURL string) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	parsedBase, err := url.Parse(baseURL)
	if err != nil || (parsedBase.Scheme != "http" && parsedBase.Scheme != "https") || parsedBase.Host == "" {
		return nil, errors.New("community server API base URL must be an absolute HTTP(S) URL")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	if strings.TrimSpace(proxyURL) != "" {
		if err := configureProxy(transport, proxyURL); err != nil {
			return nil, err
		}
	}
	return &Client{
		baseURL: strings.TrimRight(parsedBase.String(), "/"),
		http: &http.Client{
			Timeout:   12 * time.Second,
			Transport: transport,
		},
	}, nil
}

func configureProxy(transport *http.Transport, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return errors.New("community server proxy must be an absolute URL")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		transport.Proxy = http.ProxyURL(parsed)
		return nil
	case "socks5", "socks5h":
		var auth *proxy.Auth
		if parsed.User != nil {
			password, _ := parsed.User.Password()
			auth = &proxy.Auth{User: parsed.User.Username(), Password: password}
		}
		dialer, err := proxy.SOCKS5("tcp", parsed.Host, auth, &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second})
		if err != nil {
			return errors.New("cannot configure community server SOCKS5 proxy")
		}
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.Dial(network, address)
		}
		return nil
	default:
		return errors.New("community server proxy scheme must be http, https, socks5, or socks5h")
	}
}

func (c *Client) Fetch(ctx context.Context, query Query) ([]Server, int, error) {
	query = query.Normalize()
	endpoint, err := url.Parse(c.baseURL + "/servers")
	if err != nil {
		return nil, 0, errors.New("cannot build community server API request")
	}
	values := endpoint.Query()
	values.Set("filter[game]", "palworld")
	if query.Region == "cn" {
		values.Set("filter[countries][0]", "CN")
	}
	if query.Status != "all" {
		values.Set("filter[status]", query.Status)
	}
	if query.Search != "" {
		values.Set("filter[search]", query.Search)
	}
	values.Set("sort", "-players")
	values.Set("page[size]", strconv.Itoa(query.PageSize))
	values.Set("page[offset]", strconv.Itoa((query.Page-1)*query.PageSize))
	endpoint.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, 0, errors.New("cannot build community server API request")
	}
	req.Header.Set("Accept", "application/vnd.api+json, application/json")
	req.Header.Set("User-Agent", "PalPanel/community-servers")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("community server source request failed: %w", redactURLError(err))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maximumResponseBytes+1))
	if err != nil {
		return nil, 0, errors.New("cannot read community server source response")
	}
	if len(body) > maximumResponseBytes {
		return nil, 0, ErrResponseTooLarge
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("community server source returned HTTP %d", resp.StatusCode)
	}
	var payload battleMetricsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, 0, errors.New("community server source returned malformed JSON")
	}
	servers := make([]Server, 0, len(payload.Data))
	for _, item := range payload.Data {
		server := normalizeBattleMetricsServer(item)
		if server.Address == "" || server.Port <= 0 || server.Port > 65535 {
			continue
		}
		servers = append(servers, server)
	}
	total := payload.Meta.Total
	if total < len(servers) {
		total = len(servers)
	}
	return servers, total, nil
}

type battleMetricsResponse struct {
	Data []battleMetricsServer `json:"data"`
	Meta struct {
		Total int `json:"total"`
	} `json:"meta"`
}

type battleMetricsServer struct {
	ID         string         `json:"id"`
	Attributes map[string]any `json:"attributes"`
}

func normalizeBattleMetricsServer(item battleMetricsServer) Server {
	a := item.Attributes
	details := objectValue(a["details"])
	address := stringValue(a, "ip", "address")
	port := intValue(a, "port", "queryPort")
	country := strings.ToUpper(stringValue(a, "country", "countryCode"))
	if country == "" {
		country = strings.ToUpper(stringValue(details, "country", "countryCode"))
	}
	version := stringValue(a, "version")
	if version == "" {
		version = stringValue(details, "version", "gameVersion", "raw_version")
	}
	description := stringValue(a, "description")
	if description == "" {
		description = stringValue(details, "description", "serverDescription")
	}
	if description == "" {
		description = stringValue(objectValue(details["palworld"]), "description_s")
	}
	password, _ := boolValue(a, "password", "passwordProtected", "password_protected")
	if detailPassword, ok := boolValue(details, "password", "passwordProtected", "password_protected"); ok {
		password = detailPassword
	}
	updatedAt, _ := time.Parse(time.RFC3339, stringValue(a, "updatedAt"))
	return Server{
		ID: item.ID, Name: stringValue(a, "name"), Address: address, Port: port,
		Connect: net.JoinHostPort(address, strconv.Itoa(port)), Players: intValue(a, "players"),
		MaxPlayers: intValue(a, "maxPlayers", "max_players"), Password: password,
		Country: country, Version: version, Description: description,
		Status: strings.ToLower(stringValue(a, "status")), UpdatedAt: updatedAt,
	}
}

func objectValue(value any) map[string]any {
	if object, ok := value.(map[string]any); ok {
		return object
	}
	return map[string]any{}
}

func stringValue(object map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := object[key].(type) {
		case string:
			return strings.TrimSpace(value)
		case json.Number:
			return value.String()
		case float64:
			return strconv.FormatFloat(value, 'f', -1, 64)
		}
	}
	return ""
}

func intValue(object map[string]any, keys ...string) int {
	for _, key := range keys {
		switch value := object[key].(type) {
		case float64:
			return int(value)
		case json.Number:
			parsed, _ := strconv.Atoi(value.String())
			return parsed
		case string:
			parsed, _ := strconv.Atoi(value)
			return parsed
		}
	}
	return 0
}

func boolValue(object map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		switch value := object[key].(type) {
		case bool:
			return value, true
		case string:
			parsed, err := strconv.ParseBool(value)
			return parsed, err == nil
		case float64:
			return value != 0, true
		}
	}
	return false, false
}

func redactURLError(err error) error {
	var target *url.Error
	if !errors.As(err, &target) {
		return err
	}
	clone := *target
	if parsed, parseErr := url.Parse(clone.URL); parseErr == nil {
		parsed.User = nil
		clone.URL = parsed.String()
	}
	return &clone
}
