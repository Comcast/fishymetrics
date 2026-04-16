package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

func Test_ScrapeHandler_CredentialsScriptErrorReturnsGenericError(t *testing.T) {
	script := writeCredentialsScript(t, "echo raw-secret-output\nexit 42\n")

	cfg := &ScrapeConfig{CredentialsScript: script}
	h := ScrapeHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/scrape?target=1.2.3.4&model=dl360", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rr.Body.String(), credentialsRetrievalFailure) {
		t.Fatalf("body = %q, want generic credentials failure", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "raw-secret-output") || strings.Contains(rr.Body.String(), "exit status") {
		t.Fatalf("body leaked script detail: %q", rr.Body.String())
	}
}

func Test_ScrapeHandler_CredentialsScriptParseErrorReturnsGenericError(t *testing.T) {
	script := writeCredentialsScript(t, "echo 'not-json-with-secret-password'\n")

	cfg := &ScrapeConfig{CredentialsScript: script}
	h := ScrapeHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/scrape?target=1.2.3.4&model=dl360", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rr.Body.String(), credentialsRetrievalFailure) {
		t.Fatalf("body = %q, want generic credentials failure", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "not-json") || strings.Contains(rr.Body.String(), "invalid character") {
		t.Fatalf("body leaked parsing detail: %q", rr.Body.String())
	}
}

func Test_ScrapeHandler_CredentialsScriptRejectsInvalidTarget(t *testing.T) {
	script := writeCredentialsScript(t, "echo '{\"user\":\"root\",\"pass\":\"toor\"}'\n")

	cfg := &ScrapeConfig{CredentialsScript: script}
	h := ScrapeHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/scrape?target=-bad.example&model=dl360", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rr.Body.String(), "invalid target parameter") {
		t.Fatalf("body = %q, want invalid target error", rr.Body.String())
	}
}

func Test_PartialScrapeHandler_UsesCredentialsScript(t *testing.T) {
	script := writeCredentialsScript(t, "echo raw-secret-output\nexit 42\n")

	cfg := &ScrapeConfig{CredentialsScript: script}
	h := PartialScrapeHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/scrape/partial?target=1.2.3.4&components=thermal&model=dl360", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rr.Body.String(), credentialsRetrievalFailure) {
		t.Fatalf("body = %q, want generic credentials failure", rr.Body.String())
	}
}

func TestValidateCredentialsScriptTarget(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		wantErr bool
	}{
		{name: "ipv4", target: "10.2.1.42"},
		{name: "ipv6", target: "2001:db8::1"},
		{name: "bracketed ipv6", target: "[2001:db8::1]"},
		{name: "bracketed ipv6 with port", target: "[2001:db8::1]:443"},
		{name: "domain", target: "bmc-01.example.com"},
		{name: "single-label domain", target: "server1"},
		{name: "host with port", target: "server1:8443"},
		{name: "url", target: "https://bmc-01.example.com:8443/"},
		{name: "leading hyphen", target: "-bad.example.com", wantErr: true},
		{name: "hyphen label", target: "good.-bad.example.com", wantErr: true},
		{name: "underscore", target: "bad_host.example.com", wantErr: true},
		{name: "path", target: "bmc-01.example.com/redfish", wantErr: true},
		{name: "bad port", target: "bmc-01.example.com:bad", wantErr: true},
		{name: "bracketed domain", target: "[bmc-01.example.com]", wantErr: true},
		{name: "bracketed domain with port", target: "[bmc-01.example.com]:443", wantErr: true},
		{name: "bracketed single-label domain", target: "[server1]", wantErr: true},
		{name: "url with path", target: "https://bmc-01.example.com/redfish", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCredentialsScriptTarget(tt.target)
			if tt.wantErr && err == nil {
				t.Fatalf("validateCredentialsScriptTarget(%q) returned nil error", tt.target)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateCredentialsScriptTarget(%q) error = %v", tt.target, err)
			}
		})
	}
}

func TestCredentialsFromScript(t *testing.T) {
	script := writeCredentialsScript(t, "printf '{\"user\":\"root\",\"pass\":\"toor\"}'\n")

	credential, out, err := credentialsFromScript(context.Background(), script, "1.2.3.4")
	if err != nil {
		t.Fatalf("credentialsFromScript returned error: %v", err)
	}
	if credential.User != "root" || credential.Pass != "toor" {
		t.Fatalf("credential = %#v, want root/toor", credential)
	}
	if string(out.stdout) != `{"user":"root","pass":"toor"}` {
		t.Fatalf("stdout = %q", out.stdout)
	}
}

func TestCredentialsFromScriptCapsOutput(t *testing.T) {
	script := writeCredentialsScript(t, "printf '%s' '"+strings.Repeat("x", credentialsScriptMaxOutputBytes+1)+"'\n")

	_, out, err := credentialsFromScript(context.Background(), script, "1.2.3.4")
	if !errors.Is(err, errCredentialsScriptOutputTooLarge) {
		t.Fatalf("error = %v, want %v", err, errCredentialsScriptOutputTooLarge)
	}
	if len(out.stdout) != credentialsScriptMaxOutputBytes {
		t.Fatalf("stdout length = %d, want %d", len(out.stdout), credentialsScriptMaxOutputBytes)
	}
}

func TestCredentialsFromScriptRejectsMissingFields(t *testing.T) {
	script := writeCredentialsScript(t, "printf '{\"user\":\"root\"}'\n")

	_, _, err := credentialsFromScript(context.Background(), script, "1.2.3.4")
	if err == nil {
		t.Fatal("credentialsFromScript returned nil error for missing password")
	}
}

func writeCredentialsScript(t *testing.T, body string) string {
	t.Helper()

	script := filepath.Join(t.TempDir(), "credentials.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return script
}
