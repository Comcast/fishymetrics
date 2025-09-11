package exporter

import (
    "io"
    "net/http"
    "net/http/httptest"
    "net/url"
    "strings"
    "sync/atomic"
    "testing"

    "github.com/comcast/fishymetrics/common"
)

// Ensure context override proxy is used for HTTP requests.
func Test_NewHTTPClient_UsesOverrideProxy(t *testing.T) {
    var proxyHits int32

    proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&proxyHits, 1)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte(`{"ok":true}`))
    }))
    defer proxy.Close()

    ctx := WithProxyURL(nil, proxy.URL)
    client := NewHTTPClient(ctx)

    uri := "http://unreachable.example/redfish/v1/Chassis/"
    req, err := common.BuildRequest(uri, "unreachable.example")
    if err != nil {
        t.Fatalf("BuildRequest error: %v", err)
    }

    resp, err := common.DoRequest(client, req)
    if err != nil {
        t.Fatalf("DoRequest error: %v", err)
    }
    defer common.EmptyAndCloseBody(resp)

    if got := atomic.LoadInt32(&proxyHits); got != 1 {
        t.Fatalf("proxy hits = %d, want 1", got)
    }

    body, _ := io.ReadAll(resp.Body)
    if strings.TrimSpace(string(body)) != `{"ok":true}` {
        t.Fatalf("unexpected body: %s", string(body))
    }
}

// Ensure override proxy is used even if NO_PROXY would otherwise bypass.
func Test_NewHTTPClient_OverrideBeatsNoProxy(t *testing.T) {
    var proxyHits int32

    // A proxy that must be used
    proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt32(&proxyHits, 1)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte(`{"ok":true}`))
    }))
    defer proxy.Close()

    // Target URL string we will request
    target := "http://example.internal/redfish/v1/Chassis/"
    u, _ := url.Parse(target)

    // Set NO_PROXY to match target host, but provide override
    t.Setenv("NO_PROXY", u.Host)
    t.Setenv("no_proxy", u.Host)

    ctx := WithProxyURL(nil, proxy.URL)
    client := NewHTTPClient(ctx)

    req, err := common.BuildRequest(target, u.Host)
    if err != nil {
        t.Fatalf("BuildRequest error: %v", err)
    }

    resp, err := common.DoRequest(client, req)
    if err != nil {
        t.Fatalf("DoRequest error: %v", err)
    }
    defer common.EmptyAndCloseBody(resp)

    if got := atomic.LoadInt32(&proxyHits); got != 1 {
        t.Fatalf("proxy hits = %d, want 1 (override should win)", got)
    }
}

