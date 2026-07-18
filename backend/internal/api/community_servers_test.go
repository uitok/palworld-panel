package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"palpanel/internal/appconfig"
	"palpanel/internal/communityservers"
)

type apiCommunityFetcher struct{}

func (apiCommunityFetcher) Fetch(context.Context, communityservers.Query) ([]communityservers.Server, int, error) {
	return []communityservers.Server{{ID: "cn-1", Name: "中文房间", Address: "127.0.0.1", Port: 8211}}, 1, nil
}

func TestCommunityServersHandlerListAndValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, err := communityservers.New(communityservers.Options{Fetcher: apiCommunityFetcher{}})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewCommunityServersHandler(service)
	router := gin.New()
	router.GET("/api/community-servers", handler.List)
	router.GET("/api/community-servers/source-status", handler.SourceStatus)
	router.POST("/api/community-servers/refresh", handler.Refresh)

	request := httptest.NewRequest(http.MethodGet, "/api/community-servers?region=cn&page=1&page_size=20&password=false", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		OK   bool                    `json:"ok"`
		Data communityservers.Result `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || !response.OK || len(response.Data.Servers) != 1 {
		t.Fatalf("response=%#v err=%v", response, err)
	}

	invalid := httptest.NewRecorder()
	router.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/api/community-servers?page=zero", nil))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status=%d body=%s", invalid.Code, invalid.Body.String())
	}
}

func TestCommunityServerSharedRoutesDisabledAndFailureMapping(t *testing.T) {
	server := Server{cfg: appconfig.Config{
		CommunityServersAPIBaseURL: "https://mirror.example.test",
		CommunityServersProxyURL:   "socks5h://127.0.0.1:10808",
		CommunityServersRateLimit:  30,
	}}
	router := gin.New()
	router.GET("/list", server.listCommunityServers)
	router.POST("/refresh", server.refreshCommunityServers)
	router.GET("/status", server.communityServersSourceStatus)
	router.POST("/bot", server.astrBotCommunityServers)
	router.GET("/rate-limit", func(c *gin.Context) { communityServerFailure(c, communityservers.ErrRateLimited) })
	router.GET("/upstream", func(c *gin.Context) { communityServerFailure(c, errors.New("upstream failed")) })

	for _, test := range []struct {
		method string
		path   string
		body   string
		status int
		code   string
	}{
		{http.MethodGet, "/list", "", http.StatusServiceUnavailable, "community_servers_disabled"},
		{http.MethodPost, "/refresh", "", http.StatusServiceUnavailable, "community_servers_disabled"},
		{http.MethodPost, "/bot", `{}`, http.StatusServiceUnavailable, "community_servers_disabled"},
		{http.MethodGet, "/rate-limit", "", http.StatusTooManyRequests, "community_servers_rate_limited"},
		{http.MethodGet, "/upstream", "", http.StatusBadGateway, "community_servers_unavailable"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		if test.body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		router.ServeHTTP(recorder, request)
		if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.code) {
			t.Errorf("%s %s = %d %s", test.method, test.path, recorder.Code, recorder.Body.String())
		}
	}

	status := httptest.NewRecorder()
	router.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/status", nil))
	for _, value := range []string{`"enabled":false`, `"proxy_configured":true`, `"rate_limit_per_minute":30`} {
		if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), value) {
			t.Fatalf("disabled source status = %d %s", status.Code, status.Body.String())
		}
	}
}

func TestCommunityServerQueryRejectsEveryInvalidScalar(t *testing.T) {
	service, err := communityservers.New(communityservers.Options{Fetcher: apiCommunityFetcher{}})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/servers", NewCommunityServersHandler(service).List)
	for _, query := range []string{
		"min_players=-1", "max_players=bad", "page=0", "page_size=bad", "password=maybe",
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/servers?"+query, nil))
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "community_servers_invalid_query") {
			t.Errorf("query %q = %d %s", query, recorder.Code, recorder.Body.String())
		}
	}
}
