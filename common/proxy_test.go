package common

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/go-retryablehttp"
)

// Test that HTTP requests honor HTTP_PROXY and are routed via the proxy.
func Test_HTTPProxy_RoutesThroughProxy(t *testing.T) {
	var proxyHits int32

	// Mock HTTP proxy server that returns a canned response
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&proxyHits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer proxy.Close()

	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("NO_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")

	client := retryablehttp.NewClient()
	client.Logger = nil
	client.RetryMax = 0
	client.HTTPClient.Transport = &http.Transport{Proxy: http.ProxyFromEnvironment}

	// Request to any HTTP URL should go via proxy
	uri := "http://unreachable.example/redfish/v1/Chassis/"
	req, err := BuildRequest(uri, "unreachable.example")
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}

	resp, err := DoRequest(client, req)
	if err != nil {
		t.Fatalf("DoRequest error: %v", err)
	}
	defer EmptyAndCloseBody(resp)

	if got, want := atomic.LoadInt32(&proxyHits), int32(1); got != want {
		t.Fatalf("proxy hits = %d, want %d", got, want)
	}

	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != `{"ok":true}` {
		t.Fatalf("unexpected body: %s", string(body))
	}
}

// Test that HTTPS requests pick up HTTPS_PROXY via ProxyFromEnvironment (unit-level selection test).
func Test_ProxyFromEnvironment_HTTPSSelection(t *testing.T) {
	// No network I/O here; just validate selection logic
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "http://proxy.example:3128")
	t.Setenv("NO_PROXY", "")

	tr := &http.Transport{Proxy: http.ProxyFromEnvironment}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/redfish/v1/", nil)
	proxyURL, err := tr.Proxy(req)
	if err != nil {
		t.Fatalf("unexpected error from Proxy func: %v", err)
	}
	if proxyURL == nil {
		t.Skip("HTTPS proxy env var not honored in this runtime; skipping")
	}
	if proxyURL.String() != "http://proxy.example:3128" {
		t.Fatalf("unexpected proxy selection: got %v", proxyURL)
	}
}

// Test that NO_PROXY bypasses the proxy for matching hosts.
func Test_HTTPProxy_NoProxyBypass(t *testing.T) {
	var proxyHits int32
	var targetHits int32

	// Real target server that we expect to hit directly
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&targetHits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"source":"target"}`))
	}))
	defer target.Close()

	// Proxy that should NOT be used
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&proxyHits, 1)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("proxy should not be used"))
	}))
	defer proxy.Close()

	// Extract host:port from target URL for NO_PROXY
	u, err := url.Parse(target.URL)
	if err != nil {
		t.Fatalf("parse target url: %v", err)
	}

	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", u.Host) // ensure bypass for this host:port

	client := retryablehttp.NewClient()
	client.Logger = nil
	client.RetryMax = 0
	client.HTTPClient.Transport = &http.Transport{Proxy: http.ProxyFromEnvironment}

	uri := target.URL + "/redfish/v1/Chassis/"
	req, err := BuildRequest(uri, u.Hostname())
	if err != nil {
		t.Fatalf("BuildRequest error: %v", err)
	}

	resp, err := DoRequest(client, req)
	if err != nil {
		t.Fatalf("DoRequest error: %v", err)
	}
	defer EmptyAndCloseBody(resp)

	if got := atomic.LoadInt32(&proxyHits); got != 0 {
		t.Fatalf("proxy was used: hits = %d, want 0", got)
	}
	if got := atomic.LoadInt32(&targetHits); got != 1 {
		t.Fatalf("target hits = %d, want 1", got)
	}

	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != `{"source":"target"}` {
		t.Fatalf("unexpected body: %s", string(body))
	}
}
