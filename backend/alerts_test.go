package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildAlertPayloads(t *testing.T) {
	bedPayload := buildBedAlertPayload(Bed{Number: 8, Room: "A", Name: "Bed A", PatientName: "TEST_PATIENT"}, "nurse")
	if bedPayload.Reason != "bed_alert" {
		t.Fatalf("expected bed alert reason")
	}
	if bedPayload.Patient != "TEST_PATIENT" {
		t.Fatalf("expected patient name in bed payload")
	}

	triagePayload := buildTriageAlertPayload(Patient{Name: "TEST_USER", RegistrationNumber: "TEST-1"}, nil, "triage")
	if triagePayload.Reason != "triage_max" {
		t.Fatalf("expected triage_max reason")
	}
	if triagePayload.Room == "" || triagePayload.Bed == "" {
		t.Fatalf("expected default room/bed in triage payload")
	}
}

func TestSendGotifyAlertPayloadSuccessAndFailures(t *testing.T) {
	var seenPath string
	var seenToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenToken = r.URL.Query().Get("token")
		if got := r.Header.Get("X-Gotify-Key"); got != "abc" {
			t.Fatalf("unexpected gotify key header: %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode gotify body: %v", err)
		}
		if body["title"] != "URGENT" {
			t.Fatalf("expected title URGENT")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := sendGotifyAlertPayload(alertPayload{Title: "URGENT", Patient: "A", Room: "R", Bed: "B", TimeHM: "10:10", SourceUser: "u"}, gotifyConfig{Enabled: true, URL: server.URL, Token: "abc", Priority: 9})
	if err != nil {
		t.Fatalf("expected gotify send success, got error: %v", err)
	}
	if seenPath != "/message" {
		t.Fatalf("expected /message path, got %q", seenPath)
	}
	if seenToken != "abc" {
		t.Fatalf("expected token query parameter to be set")
	}

	err = sendGotifyAlertPayload(alertPayload{Title: "URGENT"}, gotifyConfig{Enabled: false})
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected disabled error")
	}

	err = sendGotifyAlertPayload(alertPayload{Title: "URGENT"}, gotifyConfig{Enabled: true, URL: "https://example.com", Token: ""})
	if err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("expected token error")
	}
}
