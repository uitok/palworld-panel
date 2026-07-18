package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"palpanel/internal/communityservers"
)

type CommunityServersHandler struct {
	service *communityservers.Service
}

func NewCommunityServersHandler(service *communityservers.Service) *CommunityServersHandler {
	return &CommunityServersHandler{service: service}
}

func (h *CommunityServersHandler) List(c *gin.Context) {
	query, valid := communityServerQuery(c)
	if !valid {
		return
	}
	result, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		communityServerFailure(c, err)
		return
	}
	ok(c, result)
}

func (h *CommunityServersHandler) Refresh(c *gin.Context) {
	query, valid := communityServerQuery(c)
	if !valid {
		return
	}
	result, err := h.service.Refresh(c.Request.Context(), query)
	if err != nil {
		communityServerFailure(c, err)
		return
	}
	ok(c, result)
}

func (h *CommunityServersHandler) SourceStatus(c *gin.Context) {
	ok(c, h.service.Status())
}

func communityServerQuery(c *gin.Context) (communityservers.Query, bool) {
	query := communityservers.Query{
		Region: c.DefaultQuery("region", "cn"), Search: c.Query("search"), Version: c.Query("version"),
		Status: c.DefaultQuery("status", "online"),
	}
	var err error
	if query.MinPlayers, err = optionalNonNegativeInt(c.Query("min_players")); err != nil {
		fail(c, http.StatusBadRequest, "community_servers_invalid_query", "min_players must be a non-negative integer")
		return query, false
	}
	if query.MaxPlayers, err = optionalNonNegativeInt(c.Query("max_players")); err != nil {
		fail(c, http.StatusBadRequest, "community_servers_invalid_query", "max_players must be a non-negative integer")
		return query, false
	}
	if query.Page, err = optionalPositiveInt(c.Query("page")); err != nil {
		fail(c, http.StatusBadRequest, "community_servers_invalid_query", "page must be a positive integer")
		return query, false
	}
	if query.PageSize, err = optionalPositiveInt(c.Query("page_size")); err != nil {
		fail(c, http.StatusBadRequest, "community_servers_invalid_query", "page_size must be a positive integer")
		return query, false
	}
	if raw := c.Query("password"); raw != "" {
		value, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			fail(c, http.StatusBadRequest, "community_servers_invalid_query", "password must be true or false")
			return query, false
		}
		query.Password = &value
	}
	return query.Normalize(), true
}

func optionalNonNegativeInt(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, errors.New("invalid non-negative integer")
	}
	return value, nil
}

func optionalPositiveInt(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, errors.New("invalid positive integer")
	}
	return value, nil
}

func communityServerFailure(c *gin.Context, err error) {
	if errors.Is(err, communityservers.ErrRateLimited) {
		fail(c, http.StatusTooManyRequests, "community_servers_rate_limited", err.Error())
		return
	}
	fail(c, http.StatusBadGateway, "community_servers_unavailable", err.Error())
}
