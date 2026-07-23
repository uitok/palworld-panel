package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"palpanel/internal/communityservers"
	"palpanel/internal/docker"
	"palpanel/internal/networkproxy"
)

type communityProxyFetcher struct {
	baseURL string
	proxy   *networkproxy.Service
}

func (f communityProxyFetcher) Fetch(ctx context.Context, query communityservers.Query) ([]communityservers.Server, int, error) {
	rawProxy := ""
	if f.proxy != nil {
		var err error
		rawProxy, err = f.proxy.CommunityProxyURL()
		if err != nil {
			return nil, 0, err
		}
	}
	client, err := communityservers.NewClient(f.baseURL, rawProxy)
	if err != nil {
		return nil, 0, err
	}
	return client.Fetch(ctx, query)
}

func (s Server) getNetworkProxyConfig(c *gin.Context) {
	config, err := s.networkProxy.Config()
	if err != nil {
		fail(c, http.StatusInternalServerError, "network_proxy_read_failed", err.Error())
		return
	}
	ok(c, config)
}

func (s Server) putNetworkProxyConfig(c *gin.Context) {
	var request networkproxy.ConfigUpdate
	if err := c.ShouldBindJSON(&request); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	config, err := s.networkProxy.Update(request)
	if err != nil {
		failNetworkProxy(c, err, "network_proxy_write_failed")
		return
	}
	ok(c, config)
}

func (s Server) testNetworkProxy(c *gin.Context) {
	var request networkproxy.TestRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	result, err := s.networkProxy.Test(c.Request.Context(), request.Scope)
	if err != nil {
		var validationErr *networkproxy.ValidationError
		if errors.As(err, &validationErr) {
			fail(c, http.StatusBadRequest, "network_proxy_invalid", validationErr.Message)
			return
		}
		result.OK = false
		result.Message = err.Error()
		ok(c, result)
		return
	}
	if request.Scope == "install" {
		container, containerErr := docker.NewRunner(s.cfg).TestInstallProxy(c.Request.Context(), result.Target)
		result.DockerOK = container.OK
		result.DockerLatencyMS = container.Latency.Milliseconds()
		result.HostNetwork = container.HostNetwork
		result.BridgeEnabled = container.BridgeEnabled
		result.Diagnostic = container.Diagnostic
		if containerErr != nil {
			result.OK = false
			result.FailureStage = container.FailureStage
			result.Message = containerErr.Error()
			ok(c, result)
			return
		}
		result.OK = result.HostOK && result.DockerOK
		result.LatencyMS += result.DockerLatencyMS
		result.Message = "host and Docker container proxy tests passed"
	}
	ok(c, result)
}

func failNetworkProxy(c *gin.Context, err error, fallbackCode string) {
	var validationErr *networkproxy.ValidationError
	if errors.As(err, &validationErr) {
		fail(c, http.StatusBadRequest, "network_proxy_invalid", validationErr.Message)
		return
	}
	fail(c, http.StatusInternalServerError, fallbackCode, err.Error())
}
