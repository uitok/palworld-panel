package palrest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL  string
	User     string
	Password string
	Client   *http.Client
}

type Response struct {
	Status int    `json:"status"`
	Body   any    `json:"body"`
	Raw    string `json:"raw,omitempty"`
}

func New(baseURL, user, password string) Client {
	return Client{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		User:     user,
		Password: password,
		Client:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (c Client) Do(ctx context.Context, method, path string, payload any) (Response, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return Response{}, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+"/"+strings.TrimLeft(path, "/"), body)
	if err != nil {
		return Response{}, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Password != "" {
		req.SetBasicAuth(c.User, c.Password)
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}
	out := Response{Status: resp.StatusCode}
	if len(b) > 0 {
		var decoded any
		if err := json.Unmarshal(b, &decoded); err == nil {
			out.Body = decoded
		} else {
			out.Raw = string(b)
			out.Body = string(b)
		}
	}
	if resp.StatusCode >= 400 {
		return out, fmt.Errorf("palworld rest api returned status %d", resp.StatusCode)
	}
	return out, nil
}
