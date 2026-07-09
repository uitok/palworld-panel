package mods

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"palpanel/internal/appconfig"
)

func TestSteamClientQueryFilesParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/IPublishedFileService/QueryFiles/v1/" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("key") != "key_123" {
			t.Fatalf("key = %q", query.Get("key"))
		}
		if query.Get("appid") != "1623730" || query.Get("creator_appid") != "1623730" {
			t.Fatalf("appid params = %q %q", query.Get("appid"), query.Get("creator_appid"))
		}
		if query.Get("search_text") != "pal" || query.Get("query_type") != "3" {
			t.Fatalf("search/sort params = %q %q", query.Get("search_text"), query.Get("query_type"))
		}
		if query.Get("cursor") != "abc" || query.Get("numperpage") != "12" {
			t.Fatalf("cursor/page params = %q %q", query.Get("cursor"), query.Get("numperpage"))
		}
		if query.Get("requiredtags[0]") != "QoL" {
			t.Fatalf("required tag = %q", query.Get("requiredtags[0]"))
		}
		_, _ = w.Write([]byte(`{"response":{"result":1,"total":1,"next_cursor":"next","publishedfiledetails":[{"publishedfileid":"123456","result":1,"title":"Test Mod","short_description":"Short","preview_url":"https://cdn.example/a.jpg","file_size":"2048","subscriptions":42,"time_created":100,"time_updated":200,"tags":[{"tag":"QoL"}]}]}}`))
	}))
	defer server.Close()

	client := NewSteamClient("key_123", "1623730")
	client.baseURL = server.URL

	result, err := client.QueryFiles(context.Background(), WorkshopSearchParams{
		Query:    "pal",
		Sort:     "trend",
		Cursor:   "abc",
		PageSize: 12,
		Tags:     []string{"QoL"},
	})
	if err != nil {
		t.Fatalf("QueryFiles returned error: %v", err)
	}
	if result.Total != 1 || result.NextCursor != "next" || len(result.Items) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	item := result.Items[0]
	if item.ID != "123456" || item.Title != "Test Mod" || item.FileSize != 2048 || item.TimeUpdated != 200 || len(item.Tags) != 1 {
		t.Fatalf("unexpected mapped item: %#v", item)
	}
}

func TestSteamClientPublishedFileDetailsForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/IPublishedFileService/GetDetails/v1/" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		query := r.URL.Query()
		if query.Get("key") != "detail-key" {
			t.Fatalf("key = %q", query.Get("key"))
		}
		if query.Get("appid") != "1623730" {
			t.Fatalf("appid = %q", query.Get("appid"))
		}
		if query.Get("publishedfileids[0]") != "999" {
			t.Fatalf("publishedfileids[0] = %q", query.Get("publishedfileids[0]"))
		}
		if query.Get("return_tags") != "1" || query.Get("return_short_description") != "1" || query.Get("return_metadata") != "1" {
			t.Fatalf("missing detail return params: %#v", query)
		}
		_, _ = w.Write([]byte(`{"response":{"result":1,"publishedfiledetails":[{"publishedfileid":"999","result":1,"title":"Detail","file_description":"Description","lifetime_subscriptions":"33","time_updated":"44"}]}}`))
	}))
	defer server.Close()

	client := NewSteamClient("detail-key", "1623730")
	client.baseURL = server.URL

	items, err := client.GetDetails(context.Background(), []string{"999"})
	if err != nil {
		t.Fatalf("GetDetails returned error: %v", err)
	}
	if len(items) != 1 || items[0].ID != "999" || items[0].Subscriptions != 33 || items[0].TimeUpdated != 44 {
		t.Fatalf("unexpected details: %#v", items)
	}
}

func TestSteamClientErrorMapping(t *testing.T) {
	client := NewSteamClient("", "1623730")
	if _, err := client.QueryFiles(context.Background(), WorkshopSearchParams{}); !errors.Is(err, ErrSteamAPIKeyMissing) {
		t.Fatalf("expected ErrSteamAPIKeyMissing, got %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer server.Close()

	client = NewSteamClient("key", "1623730")
	client.baseURL = server.URL
	_, err := client.QueryFiles(context.Background(), WorkshopSearchParams{})
	var steamErr SteamAPIError
	if !errors.As(err, &steamErr) || steamErr.Status != http.StatusBadGateway || steamErr.Code != "steam_http_error" {
		t.Fatalf("unexpected HTTP error mapping: %#v", err)
	}
}

func TestSteamClientTimeoutMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{"response":{"result":1}}`))
	}))
	defer server.Close()

	client := NewSteamClient("key", "1623730")
	client.baseURL = server.URL
	client.httpClient.Timeout = 10 * time.Millisecond

	_, err := client.QueryFiles(context.Background(), WorkshopSearchParams{})
	var steamErr SteamAPIError
	if !errors.As(err, &steamErr) || steamErr.Code != "steam_timeout" {
		t.Fatalf("expected timeout mapping, got %#v", err)
	}
}

func TestWorkshopServiceUsesEffectiveConfigKey(t *testing.T) {
	service := NewWorkshopService(appconfig.Config{WorkshopAppID: "1623730"})
	if service.client.apiKey == "" {
		t.Fatal("expected embedded Steam Web API key")
	}
	if len(service.client.apiKey) != 32 {
		t.Fatal("embedded Steam Web API key has invalid length")
	}
}
