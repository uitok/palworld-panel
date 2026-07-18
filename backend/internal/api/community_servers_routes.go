package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"palpanel/internal/communityservers"
)

func (s Server) listCommunityServers(c *gin.Context) {
	if s.communityAPI == nil {
		fail(c, http.StatusServiceUnavailable, "community_servers_disabled", "community server discovery is disabled")
		return
	}
	s.communityAPI.List(c)
}

func (s Server) refreshCommunityServers(c *gin.Context) {
	if s.communityAPI == nil {
		fail(c, http.StatusServiceUnavailable, "community_servers_disabled", "community server discovery is disabled")
		return
	}
	s.communityAPI.Refresh(c)
}

func (s Server) communityServersSourceStatus(c *gin.Context) {
	if s.communityAPI == nil {
		ok(c, gin.H{
			"source": "battlemetrics", "enabled": false,
			"base_url":         s.cfg.CommunityServersAPIBaseURL,
			"proxy_configured": strings.TrimSpace(s.cfg.CommunityServersProxyURL) != "",
			"reachable":        false, "cache_available": false, "cache_fresh": false,
			"cache_writable": false, "cached_queries": 0,
			"rate_limit_per_minute": s.cfg.CommunityServersRateLimit,
		})
		return
	}
	s.communityAPI.SourceStatus(c)
}

func (s Server) astrBotCommunityServers(c *gin.Context) {
	if s.community == nil {
		fail(c, http.StatusServiceUnavailable, "community_servers_disabled", "community server discovery is disabled")
		return
	}
	var request struct {
		Query   string `json:"query"`
		Limit   int    `json:"limit"`
		Country string `json:"country"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		fail(c, http.StatusBadRequest, "community_servers_invalid_query", err.Error())
		return
	}
	if request.Limit < 1 {
		request.Limit = 10
	}
	if request.Limit > 10 {
		request.Limit = 10
	}
	region := "cn"
	if country := strings.TrimSpace(request.Country); country != "" && !strings.EqualFold(country, "CN") {
		region = "global"
	}
	result, err := s.community.List(c.Request.Context(), communityservers.Query{
		Region: region, Search: request.Query, Status: "online", Page: 1, PageSize: request.Limit,
	})
	if err != nil {
		communityServerFailure(c, err)
		return
	}
	ok(c, result)
}
