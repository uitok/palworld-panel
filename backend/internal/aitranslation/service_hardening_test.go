package aitranslation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
)

func TestTranslatePreservesStructuredAndLongSource(t *testing.T) {
	received := make(chan chatRequest, 8)
	responses := make(chan string, 8)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		var request chatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		received <- request
		writeChatResponse(t, w, <-responses)
	}))
	defer provider.Close()

	service, _, cleanup := newTestServiceWithHTTPOptions(t, HTTPOptions{})
	defer cleanup()
	configureTestProvider(t, service, provider.URL, "content-key")

	longSource := strings.Repeat("长段落 **粗体** <span data-id=\"7\">值</span> & \\ path\n", 12_000)
	if len(longSource) >= 1024*1024 {
		t.Fatalf("long source fixture is too large: %d", len(longSource))
	}
	tests := []struct {
		name        string
		source      string
		translation string
	}{
		{name: "chinese", source: "This is a mod description.\nVersion v1.2.3.", translation: "这是一个模组说明。\n版本 v1.2.3。"},
		{name: "markdown", source: "# Heading\n\n- **bold**\n- [link](https://example.com?a=1&b=2)\n\n```ini\nKey=Value\n```", translation: "# 标题\n\n- **粗体**\n- [链接](https://example.com?a=1&b=2)\n\n```ini\nKey=Value\n```"},
		{name: "html", source: `<section data-name="mod"><p>Rock &amp; Roll</p><br><code>C:\\Palworld\\Mods</code></section>`, translation: `<section data-name="mod"><p>摇滚 &amp; 乐</p><br><code>C:\\Palworld\\Mods</code></section>`},
		{name: "special characters", source: "quotes: \" ' ; slash: \\ / ; JSON: {\"a\":[1,true]} ; emoji: 火🔥 ; tabs:\tend", translation: "引号: \" ' ; 斜杠: \\ / ; JSON: {\"a\":[1,true]} ; emoji: 火🔥 ; 制表符:\t结束"},
		{name: "long text", source: longSource, translation: "长文本翻译完成，段落结构已保留。"},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			responses <- test.translation
			translation, err := service.Translate(t.Context(), fmt.Sprintf("structured-%d", index), test.source, false)
			if err != nil {
				t.Fatalf("Translate returned error: %v", err)
			}
			if translation.Text != test.translation || translation.Cached {
				t.Fatalf("translation = %#v", translation)
			}
			request := <-received
			if request.Model != "test-model" || len(request.Messages) != 2 {
				t.Fatalf("request = %#v", request)
			}
			wantUserMessage := "<UNTRUSTED_MOD_TEXT>\n" + test.source + "\n</UNTRUSTED_MOD_TEXT>"
			if request.Messages[1].Role != "user" || request.Messages[1].Content != wantUserMessage {
				t.Fatalf("user message did not preserve source: got length %d, want %d", len(request.Messages[1].Content), len(wantUserMessage))
			}
		})
	}
}

func TestTranslateRejectsEmptyAndOversizedSourceWithoutProviderCall(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		writeChatResponse(t, w, "unexpected")
	}))
	defer provider.Close()

	service, _, cleanup := newTestServiceWithHTTPOptions(t, HTTPOptions{})
	defer cleanup()
	configureTestProvider(t, service, provider.URL, "input-key")

	for _, source := range []string{"", " \r\n\t "} {
		_, err := service.Translate(t.Context(), "empty", source, false)
		assertServiceError(t, err, http.StatusBadRequest, "ai_config_invalid")
	}
	_, err := service.Translate(t.Context(), "oversized", strings.Repeat("x", 1024*1024+1), false)
	assertServiceError(t, err, http.StatusBadRequest, "ai_config_invalid")
	if calls.Load() != 0 {
		t.Fatalf("provider calls = %d, want 0", calls.Load())
	}
}

func TestProviderFailuresAreClassifiedWithoutLeakingSecrets(t *testing.T) {
	const (
		apiKey       = "api-key-must-not-leak"
		headerSecret = "custom-header-must-not-leak"
	)
	tests := []struct {
		name       string
		status     int
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, body: apiKey, wantStatus: http.StatusBadGateway, wantCode: "ai_auth_failed"},
		{name: "forbidden", status: http.StatusForbidden, body: headerSecret, wantStatus: http.StatusBadGateway, wantCode: "ai_auth_failed"},
		{name: "rate limited", status: http.StatusTooManyRequests, body: apiKey, wantStatus: http.StatusTooManyRequests, wantCode: "ai_rate_limited"},
		{name: "server error", status: http.StatusInternalServerError, body: apiKey + headerSecret, wantStatus: http.StatusBadGateway, wantCode: "ai_upstream_error"},
		{name: "invalid json", status: http.StatusOK, body: `{not-json:"` + apiKey + `"}`, wantStatus: http.StatusBadGateway, wantCode: "ai_invalid_response"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Provider-Token") != headerSecret {
					t.Errorf("custom provider header was not sent")
				}
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer provider.Close()

			service, _, cleanup := newTestServiceWithHTTPOptions(t, HTTPOptions{
				CustomHeaders: http.Header{"X-Provider-Token": []string{headerSecret}},
			})
			defer cleanup()
			configureTestProvider(t, service, provider.URL, apiKey)

			_, err := service.Translate(t.Context(), "failure", "source", false)
			assertServiceError(t, err, test.wantStatus, test.wantCode)
			assertSecretsAbsent(t, err, apiKey, headerSecret)
		})
	}
}

func TestProviderDisconnectIsUnreachableAndSecretSafe(t *testing.T) {
	const apiKey = "disconnect-key-must-not-leak"
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		connection, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		_ = connection.Close()
	}))
	defer provider.Close()

	service, _, cleanup := newTestServiceWithHTTPOptions(t, HTTPOptions{})
	defer cleanup()
	configureTestProvider(t, service, provider.URL, apiKey)
	_, err := service.Translate(t.Context(), "disconnect", "source", false)
	assertServiceError(t, err, http.StatusBadGateway, "ai_unreachable")
	assertSecretsAbsent(t, err, apiKey)
}

func TestProviderTimeoutFromHTTPOptions(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		writeChatResponse(t, w, "late")
	}))
	defer provider.Close()

	service, _, cleanup := newTestServiceWithHTTPOptions(t, HTTPOptions{Timeout: 20 * time.Millisecond})
	defer cleanup()
	configureTestProvider(t, service, provider.URL, "timeout-key")
	_, err := service.Translate(t.Context(), "timeout", "source", false)
	assertServiceError(t, err, http.StatusGatewayTimeout, "ai_timeout")
}

func TestProviderCancellationWhileReadingResponse(t *testing.T) {
	responseStarted := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(responseStarted)
		select {
		case <-r.Context().Done():
		case <-time.After(200 * time.Millisecond):
		}
	}))
	defer provider.Close()

	service, _, cleanup := newTestServiceWithHTTPOptions(t, HTTPOptions{})
	defer cleanup()
	configureTestProvider(t, service, provider.URL, "cancel-key")
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := service.Translate(ctx, "cancel", "source", false)
		result <- err
	}()
	<-responseStarted
	cancel()
	err := <-result
	assertServiceError(t, err, 499, "ai_canceled")
}

func TestCustomProviderHeadersAreCopiedAndRestricted(t *testing.T) {
	headers := http.Header{
		"X-Tenant-ID": []string{"tenant-a"},
		"Api-Key":     []string{"provider-header-secret"},
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Tenant-ID") != "tenant-a" || r.Header.Get("Api-Key") != "provider-header-secret" {
			t.Errorf("custom headers = %#v", r.Header)
		}
		if r.Header.Get("Authorization") != "Bearer standard-key" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		writeChatResponse(t, w, "ok")
	}))
	defer provider.Close()

	service, _, cleanup := newTestServiceWithHTTPOptions(t, HTTPOptions{CustomHeaders: headers})
	defer cleanup()
	headers.Set("X-Tenant-ID", "mutated-after-construction")
	configureTestProvider(t, service, provider.URL, "standard-key")
	if _, err := service.Translate(t.Context(), "headers", "source", false); err != nil {
		t.Fatal(err)
	}

	blocked := []string{
		"Authorization", "Cookie", "Host", "Proxy-Authorization", "Proxy-Connection",
		"Forwarded", "X-Forwarded-For", "Connection", "Content-Length", "Transfer-Encoding",
	}
	for _, name := range blocked {
		t.Run("blocks "+name, func(t *testing.T) {
			_, err := NewWithHTTPOptions(appconfig.Config{}, nil, HTTPOptions{
				CustomHeaders: http.Header{name: []string{"blocked-secret-value"}},
			})
			assertServiceError(t, err, http.StatusBadRequest, "ai_config_invalid")
			assertSecretsAbsent(t, err, "blocked-secret-value")
		})
	}
	for name, value := range map[string]string{
		"Bad Header": "value",
		"X-Test":     "value\r\nInjected: true",
	} {
		t.Run("rejects invalid "+name, func(t *testing.T) {
			_, err := NewWithHTTPOptions(appconfig.Config{}, nil, HTTPOptions{
				CustomHeaders: http.Header{name: []string{value}},
			})
			assertServiceError(t, err, http.StatusBadRequest, "ai_config_invalid")
		})
	}
}

func TestExplicitHTTPProxyRoutesProviderRequest(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalls.Add(1)
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("target path = %q", r.URL.Path)
		}
		writeChatResponse(t, w, "proxied")
	}))
	defer target.Close()

	directTransport := http.DefaultTransport.(*http.Transport).Clone()
	directTransport.Proxy = nil
	defer directTransport.CloseIdleConnections()
	var proxyCalls atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCalls.Add(1)
		if !r.URL.IsAbs() || r.URL.Host == "" {
			t.Errorf("proxy received non-absolute URL %q", r.URL.String())
		}
		outbound := r.Clone(r.Context())
		outbound.RequestURI = ""
		response, err := directTransport.RoundTrip(outbound)
		if err != nil {
			t.Errorf("proxy round trip: %v", err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		for name, values := range response.Header {
			w.Header()[name] = append([]string(nil), values...)
		}
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	}))
	defer proxy.Close()

	service, _, cleanup := newTestServiceWithHTTPOptions(t, HTTPOptions{ProxyURL: proxy.URL})
	defer cleanup()
	configureTestProvider(t, service, target.URL, "proxy-key")
	translation, err := service.Translate(t.Context(), "proxy", "source", false)
	if err != nil || translation.Text != "proxied" {
		t.Fatalf("translation = %#v, error = %v", translation, err)
	}
	if proxyCalls.Load() != 1 || targetCalls.Load() != 1 {
		t.Fatalf("proxy calls = %d, target calls = %d", proxyCalls.Load(), targetCalls.Load())
	}
}

func TestPersistedHTTPOptionsApplyAfterRestartWithoutExposingSecrets(t *testing.T) {
	const (
		headerSecret = "persisted-header-secret-must-not-leak"
		proxySecret  = "persisted-proxy-secret-must-not-leak"
	)
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalls.Add(1)
		if r.Header.Get("X-Tenant-Token") != headerSecret {
			t.Errorf("custom header = %q", r.Header.Get("X-Tenant-Token"))
		}
		writeChatResponse(t, w, "persisted")
	}))
	defer target.Close()

	directTransport := http.DefaultTransport.(*http.Transport).Clone()
	directTransport.Proxy = nil
	defer directTransport.CloseIdleConnections()
	var proxyCalls atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCalls.Add(1)
		outbound := r.Clone(r.Context())
		outbound.RequestURI = ""
		outbound.Header.Del("Proxy-Authorization")
		response, err := directTransport.RoundTrip(outbound)
		if err != nil {
			t.Errorf("proxy round trip: %v", err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	}))
	defer proxy.Close()

	service, cfg, cleanup := newTestServiceWithHTTPOptions(t, HTTPOptions{})
	defer cleanup()
	baseURL, model, apiKey := target.URL+"/v1", "persisted-model", "persisted-api-key"
	timeout := 37
	proxyURL := strings.Replace(proxy.URL, "http://", "http://proxy-user:"+proxySecret+"@", 1)
	public, err := service.UpdateConfig(t.Context(), ConfigUpdate{
		BaseURL:        &baseURL,
		Model:          &model,
		APIKey:         &apiKey,
		TimeoutSeconds: &timeout,
		ProxyURL:       &proxyURL,
		CustomHeaders:  map[string]string{"X-Tenant-Token": headerSecret},
	})
	if err != nil {
		t.Fatal(err)
	}
	if public.TimeoutSeconds != timeout || !public.ProxyConfigured || public.ProxyURL != proxy.URL {
		t.Fatalf("public transport config = %#v", public)
	}
	if len(public.CustomHeaderNames) != 1 || public.CustomHeaderNames[0] != "X-Tenant-Token" {
		t.Fatalf("public header names = %#v", public.CustomHeaderNames)
	}
	assertSecretsAbsent(t, public, headerSecret, proxySecret, apiKey)

	info, err := os.Stat(service.httpConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("HTTP config mode = %o, want 600", info.Mode().Perm())
	}

	restarted := New(cfg, service.store)
	defer restarted.client.CloseIdleConnections()
	translation, err := restarted.Translate(t.Context(), "persisted-options", "source", true)
	if err != nil || translation.Text != "persisted" {
		t.Fatalf("translation = %#v, error = %v", translation, err)
	}
	if proxyCalls.Load() != 1 || targetCalls.Load() != 1 {
		t.Fatalf("proxy calls = %d, target calls = %d", proxyCalls.Load(), targetCalls.Load())
	}

	cleared, err := restarted.UpdateConfig(t.Context(), ConfigUpdate{ClearProxy: true, ClearCustomHeaders: true})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.ProxyConfigured || cleared.ProxyURL != "" || len(cleared.CustomHeaderNames) != 0 {
		t.Fatalf("cleared config = %#v", cleared)
	}
}

func TestPersistedHTTPOptionsRejectUnsafeUpdates(t *testing.T) {
	service, _, cleanup := newTestServiceWithHTTPOptions(t, HTTPOptions{})
	defer cleanup()
	invalidTimeout := 0
	_, err := service.UpdateConfig(t.Context(), ConfigUpdate{TimeoutSeconds: &invalidTimeout})
	assertServiceError(t, err, http.StatusBadRequest, "ai_config_invalid")

	proxyURL := "http://127.0.0.1:8080"
	_, err = service.UpdateConfig(t.Context(), ConfigUpdate{ProxyURL: &proxyURL, ClearProxy: true})
	assertServiceError(t, err, http.StatusBadRequest, "ai_config_invalid")

	_, err = service.UpdateConfig(t.Context(), ConfigUpdate{CustomHeaders: map[string]string{
		"Authorization": "forbidden-header-secret",
	}})
	assertServiceError(t, err, http.StatusBadRequest, "ai_config_invalid")
	assertSecretsAbsent(t, err, "forbidden-header-secret")
}

func TestProxyValidationAndCredentialsDoNotLeak(t *testing.T) {
	for _, proxyURL := range []string{
		"http://127.0.0.1:9000",
		"https://127.0.0.1:9000",
		"socks5://127.0.0.1:1080",
		"socks5h://127.0.0.1:1080",
	} {
		service, err := NewWithHTTPOptions(appconfig.Config{}, nil, HTTPOptions{ProxyURL: proxyURL})
		if err != nil {
			t.Fatalf("proxy URL %q was rejected: %v", proxyURL, err)
		}
		service.client.CloseIdleConnections()
	}

	for _, proxyURL := range []string{
		"relative-proxy",
		"ftp://127.0.0.1:9000",
		"http://proxy-user:proxy-password-must-not-leak@127.0.0.1:9000/path",
		"http://127.0.0.1:9000?secret=proxy-password-must-not-leak",
	} {
		_, err := NewWithHTTPOptions(appconfig.Config{}, nil, HTTPOptions{ProxyURL: proxyURL})
		assertServiceError(t, err, http.StatusBadRequest, "ai_config_invalid")
		assertSecretsAbsent(t, err, "proxy-password-must-not-leak")
	}

	var rejectingProxyCalls atomic.Int32
	rejectingProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rejectingProxyCalls.Add(1)
		if r.Method != http.MethodConnect {
			t.Errorf("proxy method = %s, want CONNECT", r.Method)
		}
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "proxy-password-must-not-leak proxy-api-key-must-not-leak")
	}))
	defer rejectingProxy.Close()
	proxyWithCredentials := strings.Replace(rejectingProxy.URL, "http://", "http://proxy-user:proxy-password-must-not-leak@", 1)
	service, _, cleanup := newTestServiceWithHTTPOptions(t, HTTPOptions{
		ProxyURL: proxyWithCredentials,
	})
	defer cleanup()
	baseURL, model, apiKey := "https://provider.invalid/v1", "test-model", "proxy-api-key-must-not-leak"
	if _, err := service.UpdateConfig(t.Context(), ConfigUpdate{BaseURL: &baseURL, Model: &model, APIKey: &apiKey}); err != nil {
		t.Fatal(err)
	}
	_, err := service.Translate(t.Context(), "proxy-error", "source", false)
	assertServiceError(t, err, http.StatusBadGateway, "ai_unreachable")
	assertSecretsAbsent(t, err, "proxy-password-must-not-leak", apiKey)
	if rejectingProxyCalls.Load() != 1 {
		t.Fatalf("rejecting proxy calls = %d, want 1", rejectingProxyCalls.Load())
	}
	if public, configErr := service.Config(t.Context()); configErr != nil {
		t.Fatal(configErr)
	} else if encoded, _ := json.Marshal(public); strings.Contains(string(encoded), "proxy-password-must-not-leak") {
		t.Fatalf("public config leaked proxy credentials: %s", encoded)
	}
}

func writeChatResponse(t *testing.T, w http.ResponseWriter, content string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"choices": []any{map[string]any{
			"message": map[string]string{"role": "assistant", "content": content},
		}},
	}); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func newTestServiceWithHTTPOptions(t *testing.T, options HTTPOptions) (*Service, appconfig.Config, func()) {
	t.Helper()
	root := t.TempDir()
	cfg := appconfig.Config{DataDir: root, DBPath: filepath.Join(root, "test.db")}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewWithHTTPOptions(cfg, store, options)
	if err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	return service, cfg, func() {
		service.client.CloseIdleConnections()
		_ = store.Close()
	}
}

func configureTestProvider(t *testing.T, service *Service, providerURL, apiKey string) {
	t.Helper()
	baseURL := strings.TrimRight(providerURL, "/") + "/v1"
	model := "test-model"
	if _, err := service.UpdateConfig(t.Context(), ConfigUpdate{BaseURL: &baseURL, Model: &model, APIKey: &apiKey}); err != nil {
		t.Fatal(err)
	}
}

func assertServiceError(t *testing.T, err error, status int, code string) {
	t.Helper()
	serviceError, ok := err.(*ServiceError)
	if !ok {
		t.Fatalf("error = %#v, want *ServiceError", err)
	}
	if serviceError.Status != status || serviceError.Code != code {
		t.Fatalf("error = %#v, want status %d code %q", serviceError, status, code)
	}
}

func assertSecretsAbsent(t *testing.T, value any, secrets ...string) {
	t.Helper()
	representations := []string{fmt.Sprint(value), fmt.Sprintf("%#v", value)}
	if encoded, err := json.Marshal(value); err == nil {
		representations = append(representations, string(encoded))
	}
	for _, representation := range representations {
		for _, secret := range secrets {
			if secret != "" && strings.Contains(representation, secret) {
				t.Fatalf("secret %q leaked through %q", secret, representation)
			}
		}
	}
}
