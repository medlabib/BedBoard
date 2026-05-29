package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateGotifyURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "empty is accepted", url: "", wantErr: false},
		{name: "valid https", url: "https://gotify.example.com", wantErr: false},
		{name: "valid http", url: "http://localhost:8080", wantErr: false},
		{name: "invalid scheme", url: "ftp://example.com", wantErr: true},
		{name: "missing host", url: "https:///message", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGotifyURL(tc.url)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for url %q", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("did not expect error for url %q: %v", tc.url, err)
			}
		})
	}
}

func TestHandleSecurityConfigPersistsTriageSLA(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	postReq := httptest.NewRequest(http.MethodPost, "/api/admin/security/config", mustJSONBody(t, map[string]any{
		"adminInitUsername":      "admin",
		"forceSecureCookie":      true,
		"trustProxyHeaders":      true,
		"enableHsts":             true,
		"hstsMaxAge":             31536000,
		"hstsIncludeSubdomains":  true,
		"hstsPreload":            false,
		"triageSlaMinutes":       20,
		"clearAdminInitPassword": false,
		"clearGotifyTokenEncKey": false,
	}))
	postReq.Header.Set("Content-Type", "application/json")
	postRes := httptest.NewRecorder()
	app.handleSecurityConfig(postRes, postReq)
	if postRes.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", postRes.Code, postRes.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/admin/security/config", nil)
	getRes := httptest.NewRecorder()
	app.handleSecurityConfig(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRes.Code)
	}

	var view securityConfigView
	if err := json.Unmarshal(getRes.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode security config: %v", err)
	}
	if view.TriageSLAMinutes != 20 {
		t.Fatalf("expected triage SLA 20, got %d", view.TriageSLAMinutes)
	}
}
