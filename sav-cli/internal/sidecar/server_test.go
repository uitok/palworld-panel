package sidecar

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthContractAndMethod(t *testing.T) {
	server := New()
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil || !payload.OK || payload.Data == nil {
		t.Fatalf("unexpected health payload: %#v, %v", payload, err)
	}

	recorder = httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/health", nil))
	if recorder.Code != http.StatusMethodNotAllowed || !strings.Contains(recorder.Body.String(), "method_not_allowed") {
		t.Fatalf("POST /health = %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestIndexValidatesRequestsAndMapsIndexerErrors(t *testing.T) {
	server := New()
	tests := []struct {
		name   string
		method string
		body   string
		status int
		code   string
	}{
		{name: "method", method: http.MethodGet, status: http.StatusMethodNotAllowed, code: "method_not_allowed"},
		{name: "json", method: http.MethodPost, body: "{", status: http.StatusBadRequest, code: "bad_request"},
		{name: "missing path", method: http.MethodPost, body: `{}`, status: http.StatusBadRequest, code: "save_path_required"},
		{name: "unknown path", method: http.MethodPost, body: `{"save_dir":"/path/that/does/not/exist"}`, status: http.StatusNotFound, code: "save_path_not_found"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, "/index", strings.NewReader(test.body))
			server.ServeHTTP(recorder, request)
			if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.code) {
				t.Fatalf("response = %d: %s", recorder.Code, recorder.Body.String())
			}
			if recorder.Header().Get("Content-Type") != "application/json; charset=utf-8" {
				t.Fatalf("Content-Type = %q", recorder.Header().Get("Content-Type"))
			}
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "  ", "value", "other"); got != "value" {
		t.Fatalf("firstNonEmpty = %q", got)
	}
}
