package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type gotifyConfig struct {
	Enabled  bool
	URL      string
	Token    string
	Priority int
}

type alertPayload struct {
	Title      string `json:"title"`
	Reason     string `json:"reason"`
	Patient    string `json:"patient"`
	Room       string `json:"room"`
	Bed        string `json:"bed"`
	SourceUser string `json:"sourceUser"`
	TimeHM     string `json:"timeHM"`
}

func buildBedAlertPayload(bed Bed, actor string) alertPayload {
	now := time.Now()
	return alertPayload{
		Title:      "URGENT",
		Reason:     "bed_alert",
		Patient:    fallback(bed.PatientName, "Patient inconnu"),
		Room:       fallback(bed.Room, "Chambre"),
		Bed:        fallback(bed.Name, fmt.Sprintf("Lit %d", bed.Number)),
		SourceUser: fallback(actor, "system"),
		TimeHM:     now.Format("15:04"),
	}
}

func buildTriageAlertPayload(patient Patient, bed *Bed, actor string) alertPayload {
	now := time.Now()
	room := "Non assignée"
	bedName := "Non assigné"
	if bed != nil {
		room = fallback(bed.Room, room)
		bedName = fallback(bed.Name, bedName)
	}
	return alertPayload{
		Title:      "URGENT",
		Reason:     "triage_max",
		Patient:    fallback(patient.Name, patient.RegistrationNumber),
		Room:       room,
		Bed:        bedName,
		SourceUser: fallback(actor, "system"),
		TimeHM:     now.Format("15:04"),
	}
}

func publishUrgentAlert(a *App, payload alertPayload) {
	a.broadcastEvent("alert.urgent", payload)
	config := a.getGotifyConfig()
	go func() {
		if err := sendGotifyAlertPayload(payload, config); err != nil {
			log.Printf("gotify send failed: %v", err)
		}
	}()
}

func sendGotifyAlertPayload(payload alertPayload, config gotifyConfig) error {
	if !config.Enabled {
		return fmt.Errorf("gotify is disabled")
	}
	baseURL := strings.TrimSpace(config.URL)
	if baseURL == "" {
		return fmt.Errorf("gotify url is empty")
	}
	token := strings.TrimSpace(config.Token)
	if token == "" {
		return fmt.Errorf("gotify token is empty")
	}
	priority := config.Priority
	if priority <= 0 {
		priority = 8
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("gotify url parse failed: %w", err)
	}
	if parsedURL.Path == "" {
		parsedURL.Path = "/message"
	} else if !strings.HasSuffix(parsedURL.Path, "/message") {
		parsedURL.Path = strings.TrimRight(parsedURL.Path, "/") + "/message"
	}
	query := parsedURL.Query()
	if strings.TrimSpace(query.Get("token")) == "" {
		query.Set("token", token)
	}
	parsedURL.RawQuery = query.Encode()

	message := fmt.Sprintf("%s | Patient: %s | Chambre: %s | Lit: %s | Heure: %s | Origine: %s",
		payload.Title,
		payload.Patient,
		payload.Room,
		payload.Bed,
		payload.TimeHM,
		payload.SourceUser,
	)

	body, err := json.Marshal(map[string]any{
		"title":    payload.Title,
		"message":  message,
		"priority": priority,
	})
	if err != nil {
		return fmt.Errorf("gotify marshal failed: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, parsedURL.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("gotify request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gotify-Key", token)

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("gotify call failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("gotify returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}
