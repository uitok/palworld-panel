package networkproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"palpanel/internal/appconfig"
)

const (
	configVersion       = 1
	maximumConfigBytes  = 32 << 10
	maximumProxyURLSize = 2048
)

type Endpoint struct {
	Enabled          bool   `json:"enabled"`
	Configured       bool   `json:"configured"`
	URL              string `json:"url"`
	Scheme           string `json:"scheme,omitempty"`
	Authentication   bool   `json:"authentication_configured"`
	Source           string `json:"source"`
	RequiresRestart  bool   `json:"requires_restart"`
	EffectiveForNext bool   `json:"effective_for_next_task"`
}

type PublicConfig struct {
	Install   Endpoint `json:"install"`
	Community Endpoint `json:"community"`
}

type ConfigUpdate struct {
	InstallEnabled      *bool   `json:"install_enabled,omitempty"`
	InstallProxyURL     *string `json:"install_proxy_url,omitempty"`
	ClearInstallProxy   bool    `json:"clear_install_proxy,omitempty"`
	CommunityEnabled    *bool   `json:"community_enabled,omitempty"`
	CommunityProxyURL   *string `json:"community_proxy_url,omitempty"`
	ClearCommunityProxy bool    `json:"clear_community_proxy,omitempty"`
}

type TestRequest struct {
	Scope string `json:"scope"`
}

type TestResult struct {
	OK              bool   `json:"ok"`
	Scope           string `json:"scope"`
	Target          string `json:"target"`
	LatencyMS       int64  `json:"latency_ms"`
	HTTPStatus      int    `json:"http_status"`
	ProxyScheme     string `json:"proxy_scheme"`
	ProxyEnabled    bool   `json:"proxy_enabled"`
	Message         string `json:"message"`
	HostOK          bool   `json:"host_ok"`
	HostLatencyMS   int64  `json:"host_latency_ms"`
	DockerOK        bool   `json:"docker_ok,omitempty"`
	DockerLatencyMS int64  `json:"docker_latency_ms,omitempty"`
	FailureStage    string `json:"failure_stage,omitempty"`
	Diagnostic      string `json:"diagnostic,omitempty"`
	HostNetwork     bool   `json:"host_network,omitempty"`
	BridgeEnabled   bool   `json:"bridge_enabled,omitempty"`
}

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

type persistedEndpoint struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url,omitempty"`
}

type persistedConfig struct {
	Version   int               `json:"version"`
	Install   persistedEndpoint `json:"install"`
	Community persistedEndpoint `json:"community"`
}

type resolvedConfig struct {
	persistedConfig
	InstallSource   string
	CommunitySource string
}

type Service struct {
	cfg appconfig.Config
	mu  sync.Mutex
}

func New(cfg appconfig.Config) *Service { return &Service{cfg: cfg} }

func (s *Service) Config() (PublicConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	config, err := s.readLocked()
	if err != nil {
		return PublicConfig{}, err
	}
	return publicConfig(config), nil
}

func (s *Service) Update(update ConfigUpdate) (PublicConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.readLocked()
	if err != nil {
		return PublicConfig{}, err
	}
	next := current.persistedConfig
	next.Version = configVersion
	if update.ClearInstallProxy && update.InstallProxyURL != nil && strings.TrimSpace(*update.InstallProxyURL) != "" {
		return PublicConfig{}, validation("install_proxy_url and clear_install_proxy cannot be used together")
	}
	if update.ClearCommunityProxy && update.CommunityProxyURL != nil && strings.TrimSpace(*update.CommunityProxyURL) != "" {
		return PublicConfig{}, validation("community_proxy_url and clear_community_proxy cannot be used together")
	}
	if update.InstallEnabled != nil {
		next.Install.Enabled = *update.InstallEnabled
	}
	if update.CommunityEnabled != nil {
		next.Community.Enabled = *update.CommunityEnabled
	}
	if update.ClearInstallProxy {
		next.Install.URL = ""
		next.Install.Enabled = false
	} else if update.InstallProxyURL != nil && strings.TrimSpace(*update.InstallProxyURL) != "" {
		next.Install.URL, err = ValidateURL(*update.InstallProxyURL)
		if err != nil {
			return PublicConfig{}, err
		}
	}
	if update.ClearCommunityProxy {
		next.Community.URL = ""
		next.Community.Enabled = false
	} else if update.CommunityProxyURL != nil && strings.TrimSpace(*update.CommunityProxyURL) != "" {
		next.Community.URL, err = ValidateURL(*update.CommunityProxyURL)
		if err != nil {
			return PublicConfig{}, err
		}
	}
	if next.Install.Enabled && strings.TrimSpace(next.Install.URL) == "" {
		return PublicConfig{}, validation("install proxy cannot be enabled before a proxy URL is configured")
	}
	if next.Community.Enabled && strings.TrimSpace(next.Community.URL) == "" {
		return PublicConfig{}, validation("community proxy cannot be enabled before a proxy URL is configured")
	}
	if err := s.writeLocked(next); err != nil {
		return PublicConfig{}, err
	}
	return publicConfig(resolvedConfig{persistedConfig: next, InstallSource: "managed", CommunitySource: "managed"}), nil
}

func (s *Service) InstallProxyURL() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	config, err := s.readLocked()
	if err != nil || !config.Install.Enabled {
		return "", err
	}
	return config.Install.URL, nil
}

func (s *Service) CommunityProxyURL() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	config, err := s.readLocked()
	if err != nil || !config.Community.Enabled {
		return "", err
	}
	return config.Community.URL, nil
}

func (s *Service) CommunityProxyConfigured() bool {
	value, err := s.CommunityProxyURL()
	return err == nil && value != ""
}

func (s *Service) Test(ctx context.Context, scope string) (TestResult, error) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	var rawProxy, target string
	var err error
	switch scope {
	case "install":
		rawProxy, err = s.InstallProxyURL()
		target = strings.TrimSpace(s.cfg.SteamCMDDownloadURL)
		if target == "" {
			target = appconfig.DefaultSteamCMDDownloadURL
		}
	case "community":
		rawProxy, err = s.CommunityProxyURL()
		baseURL := strings.TrimSpace(s.cfg.CommunityServersAPIBaseURL)
		if baseURL == "" {
			baseURL = appconfig.DefaultCommunityServersAPIBaseURL
		}
		target = strings.TrimRight(baseURL, "/") + "/servers?filter%5Bgame%5D=palworld&page%5Bsize%5D=1"
	default:
		return TestResult{}, validation("scope must be install or community")
	}
	if err != nil {
		return TestResult{}, err
	}
	if rawProxy == "" {
		return TestResult{}, validation(scope + " proxy is not enabled")
	}
	client, err := HTTPClient(nil, rawProxy, 12*time.Second)
	if err != nil {
		return TestResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return TestResult{}, errors.New("cannot build proxy test request")
	}
	req.Header.Set("User-Agent", "PalPanel/network-proxy-test")
	if scope == "install" {
		req.Header.Set("Range", "bytes=0-0")
	} else {
		req.Header.Set("Accept", "application/vnd.api+json, application/json")
	}
	started := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(started).Milliseconds()
	parsed, _ := url.Parse(rawProxy)
	result := TestResult{Scope: scope, Target: publicURL(target), LatencyMS: latency, HostLatencyMS: latency, ProxyScheme: strings.ToLower(parsed.Scheme), ProxyEnabled: true}
	if err != nil {
		result.FailureStage = "host_upstream_proxy"
		result.Diagnostic = "proxy=" + result.ProxyScheme + " stage=" + result.FailureStage
		return result, errors.New("proxy connection test failed")
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	result.HTTPStatus = resp.StatusCode
	result.OK = resp.StatusCode >= 200 && resp.StatusCode < 400
	result.HostOK = result.OK
	if !result.OK {
		result.FailureStage = "host_target_http"
		result.Diagnostic = "proxy=" + result.ProxyScheme + " stage=" + result.FailureStage
		return result, fmt.Errorf("proxy target returned HTTP %d", resp.StatusCode)
	}
	result.Message = "proxy connection test passed"
	return result, nil
}

func (s *Service) readLocked() (resolvedConfig, error) {
	path := s.cfg.NetworkProxyConfigPath()
	if err := s.cfg.ValidateManagedPath(path, false); err != nil {
		return resolvedConfig{}, errors.New("network proxy configuration path is outside the managed runtime")
	}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		installURL := strings.TrimSpace(os.Getenv("PALPANEL_STEAMCMD_PROXY_URL"))
		communityURL := strings.TrimSpace(s.cfg.CommunityServersProxyURL)
		if installURL != "" {
			if installURL, err = ValidateURL(installURL); err != nil {
				return resolvedConfig{}, errors.New("PALPANEL_STEAMCMD_PROXY_URL is invalid")
			}
		}
		if communityURL != "" {
			if communityURL, err = ValidateURL(communityURL); err != nil {
				return resolvedConfig{}, errors.New("PALPANEL_COMMUNITY_SERVERS_PROXY_URL is invalid")
			}
		}
		return resolvedConfig{
			persistedConfig: persistedConfig{Version: configVersion, Install: persistedEndpoint{Enabled: installURL != "", URL: installURL}, Community: persistedEndpoint{Enabled: communityURL != "", URL: communityURL}},
			InstallSource:   fallbackSource(installURL), CommunitySource: fallbackSource(communityURL),
		}, nil
	}
	if err != nil {
		return resolvedConfig{}, errors.New("cannot read network proxy configuration")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > maximumConfigBytes {
		return resolvedConfig{}, errors.New("stored network proxy configuration is invalid")
	}
	decoder := json.NewDecoder(io.LimitReader(file, maximumConfigBytes+1))
	decoder.DisallowUnknownFields()
	var config persistedConfig
	if err := decoder.Decode(&config); err != nil || config.Version != configVersion {
		return resolvedConfig{}, errors.New("stored network proxy configuration is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return resolvedConfig{}, errors.New("stored network proxy configuration is invalid")
	}
	for _, endpoint := range []persistedEndpoint{config.Install, config.Community} {
		if endpoint.URL != "" {
			if _, err := ValidateURL(endpoint.URL); err != nil {
				return resolvedConfig{}, errors.New("stored network proxy configuration is invalid")
			}
		}
		if endpoint.Enabled && endpoint.URL == "" {
			return resolvedConfig{}, errors.New("stored network proxy configuration is invalid")
		}
	}
	return resolvedConfig{persistedConfig: config, InstallSource: "managed", CommunitySource: "managed"}, nil
}

func (s *Service) writeLocked(config persistedConfig) error {
	path := s.cfg.NetworkProxyConfigPath()
	if err := s.cfg.ValidateManagedPath(path, false); err != nil {
		return errors.New("network proxy configuration path is outside the managed runtime")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return errors.New("cannot create network proxy secret directory")
	}
	body, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return errors.New("cannot encode network proxy configuration")
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".network-proxy-*.tmp")
	if err != nil {
		return errors.New("cannot create network proxy configuration")
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return errors.New("cannot secure network proxy configuration")
	}
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return errors.New("cannot write network proxy configuration")
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return errors.New("cannot sync network proxy configuration")
	}
	if err := temporary.Close(); err != nil {
		return errors.New("cannot close network proxy configuration")
	}
	if err := replaceFileAtomic(temporaryPath, path); err != nil {
		return errors.New("cannot activate network proxy configuration")
	}
	return nil
}

func ValidateURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maximumProxyURLSize {
		return "", validation("proxy URL must be a non-empty absolute URL")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Opaque != "" {
		return "", validation("proxy URL must be an absolute URL")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	switch parsed.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return "", validation("proxy URL must use HTTP, HTTPS, SOCKS5, or SOCKS5H")
	}
	if (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", validation("proxy URL must not contain a path, query, or fragment")
	}
	if strings.TrimSpace(parsed.Hostname()) == "" {
		return "", validation("proxy URL host is required")
	}
	parsed.Path = ""
	return parsed.String(), nil
}

func HTTPClient(base *http.Client, rawProxy string, timeout time.Duration) (*http.Client, error) {
	transport, err := Transport(rawProxy)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	if base != nil {
		*client = *base
	}
	client.Transport = transport
	if timeout > 0 {
		client.Timeout = timeout
	} else if client.Timeout <= 0 {
		client.Timeout = 30 * time.Second
	}
	return client, nil
}

func Transport(rawProxy string) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	rawProxy = strings.TrimSpace(rawProxy)
	if rawProxy == "" {
		return transport, nil
	}
	normalized, err := ValidateURL(rawProxy)
	if err != nil {
		return nil, err
	}
	parsed, _ := url.Parse(normalized)
	switch parsed.Scheme {
	case "http", "https":
		transport.Proxy = http.ProxyURL(parsed)
	case "socks5", "socks5h":
		var auth *proxy.Auth
		if parsed.User != nil {
			password, _ := parsed.User.Password()
			auth = &proxy.Auth{User: parsed.User.Username(), Password: password}
		}
		dialer, err := proxy.SOCKS5("tcp", parsed.Host, auth, &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second})
		if err != nil {
			return nil, errors.New("cannot configure SOCKS5 proxy")
		}
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
				return contextDialer.DialContext(ctx, network, address)
			}
			return dialer.Dial(network, address)
		}
	}
	return transport, nil
}

func publicConfig(config resolvedConfig) PublicConfig {
	return PublicConfig{
		Install:   publicEndpoint(config.Install, config.InstallSource),
		Community: publicEndpoint(config.Community, config.CommunitySource),
	}
}

func publicEndpoint(endpoint persistedEndpoint, source string) Endpoint {
	parsed, _ := url.Parse(endpoint.URL)
	return Endpoint{
		Enabled: endpoint.Enabled, Configured: endpoint.URL != "", URL: publicURL(endpoint.URL),
		Scheme: strings.ToLower(parsed.Scheme), Authentication: parsed.User != nil,
		Source: source, RequiresRestart: false, EffectiveForNext: true,
	}
}

func publicURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.User = nil
	return parsed.String()
}

func validation(message string) error { return &ValidationError{Message: message} }

func fallbackSource(rawURL string) string {
	if strings.TrimSpace(rawURL) != "" {
		return "environment"
	}
	return "managed"
}
