package breeding

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"palpanel/internal/appconfig"
)

func TestStatusReportsHealthyBridge(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","upstream_version":"v1.17.6","database_version":"2026.7"}`))
	}))
	defer upstream.Close()

	service := New(appconfig.Config{PalCalcBridgeURL: upstream.URL, PalCalcTimeoutSeconds: 5}, nil, nil)
	status := service.Status(t.Context())
	if !status.Configured || !status.Available {
		t.Fatalf("status = %#v", status)
	}
	if status.UpstreamVersion != "v1.17.6" || status.DatabaseVersion != "2026.7" {
		t.Fatalf("versions = %#v", status)
	}
}

func TestStatusReportsUnavailableBridge(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "offline", http.StatusServiceUnavailable)
	}))
	upstream.Close()

	service := New(appconfig.Config{PalCalcBridgeURL: upstream.URL, PalCalcTimeoutSeconds: 1}, nil, nil)
	status := service.Status(t.Context())
	if status.Available || status.LastError == "" {
		t.Fatalf("status = %#v", status)
	}
}
