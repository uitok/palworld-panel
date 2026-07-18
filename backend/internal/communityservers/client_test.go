package communityservers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientBuildsBattleMetricsQueryAndNormalizesServers(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/servers" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("filter[game]") != "palworld" || query.Get("filter[countries][0]") != "CN" || query.Get("filter[status]") != "online" {
			t.Fatalf("unexpected filters: %v", query)
		}
		if query.Get("filter[search]") != "中文房间" || query.Get("page[size]") != "25" || query.Get("page[offset]") != "25" {
			t.Fatalf("unexpected pagination/search: %v", query)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
          "data": [
            {"id":"srv-1","attributes":{"name":"中文房间","ip":"203.0.113.9","port":8211,"players":12,"maxPlayers":32,"status":"online","country":"cn","updatedAt":"2026-07-18T08:00:00Z","details":{"password":true,"version":"v0.6.4","palworld":{"description_s":"欢迎"}}}},
            {"id":"bad","attributes":{"name":"missing endpoint","players":1}}
          ],
          "meta":{"total":42}
        }`))
	}))
	defer upstream.Close()

	client, err := NewClient(upstream.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	servers, total, err := client.Fetch(t.Context(), Query{Region: "cn", Search: "中文房间", Status: "online", Page: 2, PageSize: 25})
	if err != nil {
		t.Fatal(err)
	}
	if total != 42 || len(servers) != 1 {
		t.Fatalf("servers=%#v total=%d", servers, total)
	}
	server := servers[0]
	if server.Connect != "203.0.113.9:8211" || !server.Password || server.Country != "CN" || server.Version != "v0.6.4" || server.Description != "欢迎" {
		t.Fatalf("normalized server = %#v", server)
	}
}

func TestClientHandlesMalformedAndOversizedResponses(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "malformed", body: "{"},
		{name: "oversized", body: strings.Repeat("x", maximumResponseBytes+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(test.body)) }))
			defer upstream.Close()
			client, err := NewClient(upstream.URL, "")
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := client.Fetch(t.Context(), Query{}); err == nil {
				t.Fatal("expected response error")
			}
		})
	}
}

func TestClientValidatesBaseAndProxyWithoutLeakingCredentials(t *testing.T) {
	for _, options := range [][2]string{
		{"file:///tmp/api", ""},
		{"https://api.example.test", "ftp://secret-user:secret-password@example.test:21"},
		{"https://api.example.test", "://secret-password"},
	} {
		_, err := NewClient(options[0], options[1])
		if err == nil {
			t.Fatalf("NewClient(%q, %q) succeeded", options[0], options[1])
		}
		if strings.Contains(err.Error(), "secret-user") || strings.Contains(err.Error(), "secret-password") {
			t.Fatalf("credentials leaked in error: %v", err)
		}
	}
}
