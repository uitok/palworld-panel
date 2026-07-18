package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

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
