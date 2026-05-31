package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
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

func TestValidateProxyURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "empty is accepted", url: "", wantErr: false},
		{name: "valid http", url: "http://proxy.example.com:8080", wantErr: false},
		{name: "valid https", url: "https://proxy.example.com:8443", wantErr: false},
		{name: "invalid scheme", url: "socks5://proxy.example.com:1080", wantErr: true},
		{name: "missing host", url: "http:///", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateProxyURL(tc.url)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for url %q", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("did not expect error for url %q: %v", tc.url, err)
			}
		})
	}
}

func TestValidateIPAllowlist(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "empty accepted", value: "", wantErr: false},
		{name: "single ip", value: "127.0.0.1", wantErr: false},
		{name: "mixed ip and cidr", value: "127.0.0.1,10.0.0.0/24", wantErr: false},
		{name: "invalid ip", value: "not-an-ip", wantErr: true},
		{name: "invalid cidr", value: "10.0.0.0/99", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateIPAllowlist(tc.value)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for allowlist %q", tc.value)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("did not expect error for allowlist %q: %v", tc.value, err)
			}
		})
	}
}

func TestVerifyAlertAckCallback(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	if err := app.upsertSettingValue(settingAlertCallbackSignatureRequired, "true"); err != nil {
		t.Fatalf("save signature required: %v", err)
	}
	if err := app.upsertSettingValue(settingAlertCallbackSecret, "shared-secret"); err != nil {
		t.Fatalf("save callback secret: %v", err)
	}
	if err := app.upsertSettingValue(settingAlertCallbackIPAllowlist, "127.0.0.1"); err != nil {
		t.Fatalf("save ip allowlist: %v", err)
	}

	body := []byte(`{"token":"abc"}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := computeCallbackHMAC("shared-secret", ts, body)

	req := httptest.NewRequest(http.MethodPost, "/api/integrations/alerts/ack", mustJSONBody(t, map[string]any{"token": "abc"}))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-BedBoard-Timestamp", ts)
	req.Header.Set("X-BedBoard-Signature", "sha256="+sig)

	if err := app.verifyAlertAckCallback(req, body); err != nil {
		t.Fatalf("expected callback verification success, got %v", err)
	}

	reqBadSig := httptest.NewRequest(http.MethodPost, "/api/integrations/alerts/ack", mustJSONBody(t, map[string]any{"token": "abc"}))
	reqBadSig.RemoteAddr = "127.0.0.1:12345"
	reqBadSig.Header.Set("X-BedBoard-Timestamp", ts)
	reqBadSig.Header.Set("X-BedBoard-Signature", "sha256=deadbeef")
	if err := app.verifyAlertAckCallback(reqBadSig, body); err == nil {
		t.Fatalf("expected callback verification failure for invalid signature")
	}

	reqBadIP := httptest.NewRequest(http.MethodPost, "/api/integrations/alerts/ack", mustJSONBody(t, map[string]any{"token": "abc"}))
	reqBadIP.RemoteAddr = "203.0.113.10:12345"
	reqBadIP.Header.Set("X-BedBoard-Timestamp", ts)
	reqBadIP.Header.Set("X-BedBoard-Signature", "sha256="+sig)
	if err := app.verifyAlertAckCallback(reqBadIP, body); err == nil {
		t.Fatalf("expected callback verification failure for disallowed ip")
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
		"proxyEnabled":           true,
		"proxyUrl":               "http://proxy.example.com:8080",
		"proxyUsername":          "proxy-user",
		"proxyPassword":          "proxy-pass",
		"clearAdminInitPassword": false,
		"clearGotifyTokenEncKey": false,
		"clearProxyPassword":     false,
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
	if !view.ProxyEnabled {
		t.Fatalf("expected proxy enabled")
	}
	if view.ProxyURL != "http://proxy.example.com:8080" {
		t.Fatalf("expected proxy url to persist, got %q", view.ProxyURL)
	}
	if view.ProxyUsername != "proxy-user" {
		t.Fatalf("expected proxy username to persist, got %q", view.ProxyUsername)
	}
	if !view.ProxyPasswordConfigured {
		t.Fatalf("expected proxy password configured")
	}
}

func TestHandleUIConfigPersistsPatientViewIdentityMode(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	postReq := httptest.NewRequest(http.MethodPost, "/api/admin/ui/config", mustJSONBody(t, map[string]any{
		"appName":                 "BedBoard",
		"logoDataUrl":             "",
		"locale":                  "en",
		"patientViewIdentityMode": "number",
		"patientCallLanguage":     "ar",
		"clearLogo":               false,
	}))
	postReq.Header.Set("Content-Type", "application/json")
	postRes := httptest.NewRecorder()
	app.handleUIConfig(postRes, postReq)
	if postRes.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", postRes.Code, postRes.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/public/ui-config", nil)
	getRes := httptest.NewRecorder()
	app.handlePublicUIConfig(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRes.Code)
	}

	var view uiConfigView
	if err := json.Unmarshal(getRes.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode ui config: %v", err)
	}
	if view.PatientViewIdentityMode != "number" {
		t.Fatalf("expected patientViewIdentityMode=number, got %q", view.PatientViewIdentityMode)
	}
	if view.PatientCallLanguage != "ar" {
		t.Fatalf("expected patientCallLanguage=ar, got %q", view.PatientCallLanguage)
	}
}
