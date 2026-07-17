package astrbotclient

import (
	"strconv"
	"testing"
	"time"
)

func TestLoopbackHostDetection(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
		if !isLoopbackHost(host) {
			t.Fatalf("expected %q to be loopback", host)
		}
	}
	for _, host := range []string{"192.168.1.10", "example.com", ""} {
		if isLoopbackHost(host) {
			t.Fatalf("expected %q not to be loopback", host)
		}
	}
}

func TestVerifyRejectsTamperingAndExpiredTimestamp(t *testing.T) {
	body := []byte(`{"qq_id":"10001"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "nonce"
	supplied := sign("secret", "POST", "/v1/test", timestamp, nonce, body)
	if !Verify("secret", "POST", "/v1/test", timestamp, nonce, supplied, body) {
		t.Fatal("valid signature was rejected")
	}
	if Verify("secret", "POST", "/v1/test", timestamp, nonce, supplied, append(body, 'x')) {
		t.Fatal("tampered body was accepted")
	}
	expired := strconv.FormatInt(time.Now().Add(-2*time.Minute).Unix(), 10)
	if Verify("secret", "POST", "/v1/test", expired, nonce, sign("secret", "POST", "/v1/test", expired, nonce, body), body) {
		t.Fatal("expired timestamp was accepted")
	}
}
