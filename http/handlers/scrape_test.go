package handlers

import (
    "net/http"
    "net/http/httptest"
    "net/url"
    "testing"
)

// Ensure invalid proxy_host is rejected early with 400.
func Test_ScrapeHandler_InvalidProxyHost(t *testing.T) {
    cfg := &ScrapeConfig{}
    h := ScrapeHandler(cfg)

    req := httptest.NewRequest(http.MethodGet, "/scrape", nil)
    q := url.Values{}
    q.Set("target", "1.2.3.4")
    q.Set("model", "dl360")
    q.Set("proxy_host", "://bad") // invalid
    req.URL.RawQuery = q.Encode()

    rr := httptest.NewRecorder()
    h(rr, req)

    if rr.Code != http.StatusBadRequest {
        t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
    }
}

