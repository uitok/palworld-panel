package networkproxy

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"palpanel/internal/appconfig"
	"palpanel/internal/communityservers"
)

func TestServicePersistsAndRedactsIndependentProxyCredentials(t *testing.T) {
	root := t.TempDir()
	cfg := appconfig.Config{RuntimeRoot: root, DataDir: filepath.Join(root, "data")}
	service := New(cfg)
	empty, err := service.Config()
	if err != nil || empty.Install.Source != "managed" || empty.Community.Source != "managed" {
		t.Fatalf("empty configuration source = %#v, %v", empty, err)
	}
	install := "http://install-user:install-secret@127.0.0.1:7890"
	community := "socks5h://community-user:community-secret@127.0.0.1:10808"
	enabled := true
	public, err := service.Update(ConfigUpdate{
		InstallEnabled: &enabled, InstallProxyURL: &install,
		CommunityEnabled: &enabled, CommunityProxyURL: &community,
	})
	if err != nil {
		t.Fatal(err)
	}
	encodedBody, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(encodedBody)
	for _, secret := range []string{"install-user", "install-secret", "community-user", "community-secret"} {
		if strings.Contains(encoded, secret) {
			t.Fatalf("public configuration leaked %q: %s", secret, encoded)
		}
	}
	if public.Install.URL != "http://127.0.0.1:7890" || public.Community.URL != "socks5h://127.0.0.1:10808" || !public.Install.Authentication {
		t.Fatalf("unexpected public configuration: %#v", public)
	}
	installURL, err := New(cfg).InstallProxyURL()
	if err != nil || installURL != install {
		t.Fatalf("persisted install proxy = %q, %v", installURL, err)
	}
	body, err := os.ReadFile(cfg.NetworkProxyConfigPath())
	if err != nil || !strings.Contains(string(body), "install-secret") {
		t.Fatalf("stored configuration missing secret: %v", err)
	}
	if info, err := os.Stat(cfg.NetworkProxyConfigPath()); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0) {
		t.Fatalf("secret file permissions = %v, %v", info, err)
	}

	cleared, err := service.Update(ConfigUpdate{ClearInstallProxy: true})
	if err != nil || cleared.Install.Configured || cleared.Install.Enabled || !cleared.Community.Enabled {
		t.Fatalf("clear result = %#v, %v", cleared, err)
	}
}

func TestValidateURLRejectsUnsafeShapes(t *testing.T) {
	for _, raw := range []string{"", "ftp://127.0.0.1:21", "http://127.0.0.1:7890/path", "socks5://", "http://127.0.0.1:7890?secret=value"} {
		if _, err := ValidateURL(raw); err == nil {
			t.Fatalf("ValidateURL(%q) succeeded", raw)
		}
	}
	for _, raw := range []string{"http://127.0.0.1:7890", "https://user:pass@proxy.example:443", "socks5://127.0.0.1:10808", "socks5h://proxy.example:10808"} {
		if _, err := ValidateURL(raw); err != nil {
			t.Fatalf("ValidateURL(%q): %v", raw, err)
		}
	}
}

func TestEnvironmentFallbackIsReplacedByManagedConfiguration(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PALPANEL_STEAMCMD_PROXY_URL", "http://127.0.0.1:7890")
	cfg := appconfig.Config{RuntimeRoot: root, DataDir: filepath.Join(root, "data"), CommunityServersProxyURL: "socks5://127.0.0.1:10808"}
	service := New(cfg)
	public, err := service.Config()
	if err != nil || public.Install.Source != "environment" || !public.Install.Enabled || public.Community.Source != "environment" {
		t.Fatalf("environment fallback = %#v, %v", public, err)
	}
	managed, err := service.Update(ConfigUpdate{ClearInstallProxy: true})
	if err != nil || managed.Install.Source != "managed" || managed.Install.Configured || !managed.Community.Enabled {
		t.Fatalf("managed override = %#v, %v", managed, err)
	}
	if value, err := New(cfg).InstallProxyURL(); err != nil || value != "" {
		t.Fatalf("managed clear did not override environment: %q, %v", value, err)
	}
}

func TestHTTPClientUsesExplicitAuthenticatedProxy(t *testing.T) {
	var authorization string
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Proxy-Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer proxyServer.Close()
	parsed, _ := url.Parse(proxyServer.URL)
	parsed.User = url.UserPassword("proxy-user", "proxy-secret")
	client, err := HTTPClient(nil, parsed.String(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Get("http://unresolvable.invalid/probe")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if authorization == "" || !strings.HasPrefix(authorization, "Basic ") {
		t.Fatalf("Proxy-Authorization = %q", authorization)
	}
}

func TestBridgeChainsHTTPSConnectThroughHTTPProxy(t *testing.T) {
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "bridge-ok")
	}))
	defer target.Close()
	upstream := httptest.NewServer(http.HandlerFunc(connectingProxyHandler))
	defer upstream.Close()
	bridge, err := StartBridge(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.Close()
	bridgeURL, _ := url.Parse("http://" + bridge.Address())
	client := target.Client()
	transport := client.Transport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(bridgeURL)
	client.Transport = transport
	response, err := client.Get(target.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if string(body) != "bridge-ok" {
		t.Fatalf("body = %q", body)
	}
}

func TestLiveCommunityDiscoveryThroughManagedProxy(t *testing.T) {
	if os.Getenv("PALPANEL_LIVE_COMMUNITY_PROXY") != "1" {
		t.Skip("set PALPANEL_LIVE_COMMUNITY_PROXY=1 and PALPANEL_LIVE_COMMUNITY_PROXY_URL to run the live community proxy check")
	}
	root := t.TempDir()
	cfg := appconfig.Config{RuntimeRoot: root, DataDir: filepath.Join(root, "data"), CommunityServersAPIBaseURL: appconfig.DefaultCommunityServersAPIBaseURL, SteamCMDDownloadURL: appconfig.DefaultSteamCMDDownloadURL}
	rawProxy := strings.TrimSpace(os.Getenv("PALPANEL_LIVE_COMMUNITY_PROXY_URL"))
	enabled := true
	service := New(cfg)
	if _, err := service.Update(ConfigUpdate{CommunityEnabled: &enabled, CommunityProxyURL: &rawProxy}); err != nil {
		t.Fatal(err)
	}
	if result, err := service.Test(t.Context(), "community"); err != nil || !result.OK {
		t.Fatalf("managed proxy test = %#v, %v", result, err)
	}
	resolved, err := service.CommunityProxyURL()
	if err != nil {
		t.Fatal(err)
	}
	client, err := communityservers.NewClient(cfg.CommunityServersAPIBaseURL, resolved)
	if err != nil {
		t.Fatal(err)
	}
	servers, total, err := client.Fetch(t.Context(), communityservers.Query{Region: "cn", Status: "online", Page: 1, PageSize: 5})
	if err != nil {
		t.Fatal(err)
	}
	if total < len(servers) {
		t.Fatalf("community source total %d is smaller than returned servers %d", total, len(servers))
	}
}

func connectingProxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodConnect {
		http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
		return
	}
	upstream, err := (&net.Dialer{}).DialContext(r.Context(), "tcp", r.Host)
	if err != nil {
		http.Error(w, "dial failed", http.StatusBadGateway)
		return
	}
	hijacker := w.(http.Hijacker)
	client, buffer, err := hijacker.Hijack()
	if err != nil {
		_ = upstream.Close()
		return
	}
	_, _ = buffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
	_ = buffer.Flush()
	go func() {
		defer client.Close()
		defer upstream.Close()
		done := make(chan struct{}, 2)
		go func() { _, _ = io.Copy(upstream, client); done <- struct{}{} }()
		go func() { _, _ = io.Copy(client, upstream); done <- struct{}{} }()
		<-done
	}()
}
