package palrest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientSendsJSONAndBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/api/announce" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		user, password, ok := r.BasicAuth()
		if !ok || user != "admin" || password != "secret" {
			t.Fatalf("unexpected basic auth: %q %q %v", user, password, ok)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"message":"hello"}` {
			t.Fatalf("body = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accepted":true}`))
	}))
	defer server.Close()

	client := New(server.URL+"/v1/api/", "admin", "secret")
	response, err := client.Do(t.Context(), http.MethodPost, "/announce", map[string]string{"message": "hello"})
	if err != nil || response.Status != http.StatusOK {
		t.Fatalf("Do = %#v, %v", response, err)
	}
	body, ok := response.Body.(map[string]any)
	if !ok || body["accepted"] != true {
		t.Fatalf("unexpected response body: %#v", response.Body)
	}
}

func TestClientHandlesTextErrorsAndResponseLimits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/text":
			_, _ = w.Write([]byte("plain text"))
		case "/error":
			http.Error(w, "offline", http.StatusServiceUnavailable)
		case "/large":
			_, _ = w.Write([]byte(strings.Repeat("x", 32)))
		}
	}))
	defer server.Close()
	client := New(server.URL, "", "")

	response, err := client.Do(t.Context(), http.MethodGet, "text", nil)
	if err != nil || response.Raw != "plain text" || response.Body != "plain text" {
		t.Fatalf("text response = %#v, %v", response, err)
	}
	response, err = client.Do(t.Context(), http.MethodGet, "error", nil)
	if err == nil || response.Status != http.StatusServiceUnavailable {
		t.Fatalf("error response = %#v, %v", response, err)
	}
	response, err = client.DoWithLimit(t.Context(), http.MethodGet, "large", nil, 8)
	if !errors.Is(err, ErrResponseTooLarge) || response.Status != http.StatusOK {
		t.Fatalf("limited response = %#v, %v", response, err)
	}
}

func TestClientHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	client := New("http://127.0.0.1:1", "", "")
	_, err := client.Do(ctx, http.MethodGet, "info", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do error = %v, want context canceled", err)
	}
}
