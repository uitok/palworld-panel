package networkproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// Bridge exposes a loopback-only HTTP proxy and forwards every connection
// through a configured HTTP(S) or SOCKS5 upstream. It exists because native
// Windows SteamCMD can consume a current-user HTTP proxy but cannot reliably
// consume application-scoped SOCKS environment variables.
type Bridge struct {
	listener net.Listener
	server   *http.Server
	once     sync.Once
}

func StartBridge(rawUpstream string) (*Bridge, error) {
	normalized, err := ValidateURL(rawUpstream)
	if err != nil {
		return nil, err
	}
	upstream, _ := url.Parse(normalized)
	transport, err := Transport(normalized)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("cannot start local SteamCMD proxy bridge")
	}
	handler := &bridgeHandler{upstream: upstream, transport: transport}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	bridge := &Bridge{listener: listener, server: server}
	go func() { _ = server.Serve(listener) }()
	return bridge, nil
}

func (b *Bridge) Address() string {
	if b == nil || b.listener == nil {
		return ""
	}
	return b.listener.Addr().String()
}

func (b *Bridge) Close() error {
	if b == nil {
		return nil
	}
	var err error
	b.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err = b.server.Shutdown(ctx)
		_ = b.listener.Close()
	})
	return err
}

type bridgeHandler struct {
	upstream  *url.URL
	transport *http.Transport
}

func (h *bridgeHandler) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodConnect {
		h.connect(w, request)
		return
	}
	h.forward(w, request)
}

func (h *bridgeHandler) forward(w http.ResponseWriter, request *http.Request) {
	outbound := request.Clone(request.Context())
	outbound.RequestURI = ""
	outbound.Header = cloneHeader(request.Header)
	removeHopHeaders(outbound.Header)
	outbound.Header.Del("Proxy-Authorization")
	if outbound.URL == nil || outbound.URL.Scheme == "" || outbound.URL.Host == "" {
		http.Error(w, "absolute proxy URL required", http.StatusBadRequest)
		return
	}
	response, err := h.transport.RoundTrip(outbound)
	if err != nil {
		http.Error(w, "upstream proxy request failed", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	removeHopHeaders(response.Header)
	copyHeader(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

func (h *bridgeHandler) connect(w http.ResponseWriter, request *http.Request) {
	target := strings.TrimSpace(request.Host)
	if target == "" || !strings.Contains(target, ":") {
		http.Error(w, "CONNECT target is invalid", http.StatusBadRequest)
		return
	}
	upstream, err := dialThroughProxy(request.Context(), h.upstream, target)
	if err != nil {
		http.Error(w, "upstream proxy connection failed", http.StatusBadGateway)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = upstream.Close()
		http.Error(w, "proxy tunnel is unavailable", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		_ = upstream.Close()
		return
	}
	if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		_ = client.Close()
		_ = upstream.Close()
		return
	}
	if err := buffered.Flush(); err != nil {
		_ = client.Close()
		_ = upstream.Close()
		return
	}
	go tunnel(client, upstream)
}

func dialThroughProxy(ctx context.Context, upstream *url.URL, target string) (net.Conn, error) {
	switch upstream.Scheme {
	case "socks5", "socks5h":
		var auth *proxy.Auth
		if upstream.User != nil {
			password, _ := upstream.User.Password()
			auth = &proxy.Auth{User: upstream.User.Username(), Password: password}
		}
		dialer, err := proxy.SOCKS5("tcp", upstream.Host, auth, &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second})
		if err != nil {
			return nil, errors.New("cannot configure SOCKS5 proxy")
		}
		if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
			return contextDialer.DialContext(ctx, "tcp", target)
		}
		return dialer.Dial("tcp", target)
	case "http", "https":
		return dialHTTPProxy(ctx, upstream, target)
	default:
		return nil, errors.New("unsupported upstream proxy")
	}
}

func dialHTTPProxy(ctx context.Context, upstream *url.URL, target string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", upstream.Host)
	if err != nil {
		return nil, errors.New("cannot connect to upstream proxy")
	}
	if upstream.Scheme == "https" {
		tlsConnection := tls.Client(connection, &tls.Config{ServerName: upstream.Hostname(), MinVersion: tls.VersionTLS12})
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			_ = connection.Close()
			return nil, errors.New("cannot establish TLS with upstream proxy")
		}
		connection = tlsConnection
	}
	request := &http.Request{Method: http.MethodConnect, URL: &url.URL{Opaque: target}, Host: target, Header: make(http.Header)}
	if upstream.User != nil {
		password, _ := upstream.User.Password()
		token := base64.StdEncoding.EncodeToString([]byte(upstream.User.Username() + ":" + password))
		request.Header.Set("Proxy-Authorization", "Basic "+token)
	}
	if err := request.WriteProxy(connection); err != nil {
		_ = connection.Close()
		return nil, errors.New("cannot send upstream proxy tunnel request")
	}
	reader := bufio.NewReader(connection)
	response, err := http.ReadResponse(reader, request)
	if err != nil {
		_ = connection.Close()
		return nil, errors.New("cannot read upstream proxy tunnel response")
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_ = connection.Close()
		return nil, errors.New("upstream proxy rejected the tunnel")
	}
	if reader.Buffered() > 0 {
		return &bufferedConnection{Conn: connection, reader: reader}, nil
	}
	return connection, nil
}

type bufferedConnection struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConnection) Read(value []byte) (int, error) { return c.reader.Read(value) }

func tunnel(client, upstream net.Conn) {
	defer client.Close()
	defer upstream.Close()
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, client); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, upstream); done <- struct{}{} }()
	<-done
}

func cloneHeader(header http.Header) http.Header {
	cloned := make(http.Header, len(header))
	copyHeader(cloned, header)
	return cloned
}

func copyHeader(destination, source http.Header) {
	for name, values := range source {
		destination[name] = append([]string(nil), values...)
	}
}

func removeHopHeaders(header http.Header) {
	for _, name := range []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailer", "Transfer-Encoding", "Upgrade"} {
		header.Del(name)
	}
}
