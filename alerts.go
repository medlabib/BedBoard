package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	go sendGotifyAlertPayload(payload, config)
}

func sendGotifyAlertPayload(payload alertPayload, config gotifyConfig) {
	if !config.Enabled {
		return
	}
	baseURL := strings.TrimSpace(config.URL)
	if baseURL == "" {
		return
	}
	token := strings.TrimSpace(config.Token)
	priority := config.Priority
	if priority <= 0 {
		priority = 8
	}

	endpoint := strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(endpoint, "/message") {
		endpoint += "/message"
	}

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
		log.Printf("gotify marshal failed: %v", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		log.Printf("gotify request failed: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Gotify-Key", token)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("gotify call failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("gotify returned status %d", resp.StatusCode)
	}
}
