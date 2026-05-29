package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type gotifyConfig struct {
	Enabled       bool
	URL           string
	Token         string
	Priority      int
	ProxyEnabled  bool
	ProxyURL      string
	ProxyUsername string
	ProxyPassword string
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
			appLog.Warnw("gotify send failed", "error", err)
		}
		if err := a.sendAlertChannels(payload); err != nil {
			appLog.Warnw("outbound alert channels failed", "error", err)
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

	client, err := buildOutboundHTTPClient(config.ProxyEnabled, config.ProxyURL, config.ProxyUsername, config.ProxyPassword)
	if err != nil {
		return err
	}
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

func buildOutboundHTTPClient(proxyEnabled bool, proxyURL, proxyUsername, proxyPassword string) (*http.Client, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	if !proxyEnabled || strings.TrimSpace(proxyURL) == "" {
		return client, nil
	}
	proxyParsed, err := url.Parse(strings.TrimSpace(proxyURL))
	if err != nil {
		return nil, fmt.Errorf("proxy url parse failed: %w", err)
	}
	if strings.TrimSpace(proxyUsername) != "" {
		proxyParsed.User = url.UserPassword(strings.TrimSpace(proxyUsername), strings.TrimSpace(proxyPassword))
	}
	client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyParsed)}
	return client, nil
}

func (a *App) sendAlertChannels(payload alertPayload) error {
	channels := a.getAlertChannelsConfig()
	proxyConfig := a.getGotifyConfig()

	type channelTarget struct {
		Name      string
		Enabled   bool
		Webhook   string
		Recipient string
	}

	targets := []channelTarget{
		{Name: "sms", Enabled: channels.SMS.Enabled, Webhook: channels.SMS.WebhookURL, Recipient: channels.SMS.Recipient},
		{Name: "whatsapp", Enabled: channels.WhatsApp.Enabled, Webhook: channels.WhatsApp.WebhookURL, Recipient: channels.WhatsApp.Recipient},
	}

	hadEnabled := false
	hadSuccess := false
	var errs []string
	for _, target := range targets {
		if !target.Enabled {
			continue
		}
		hadEnabled = true
		ackToken, err := randomToken()
		if err != nil {
			err = fmt.Errorf("%s token generation failed", target.Name)
			errs = append(errs, err.Error())
			continue
		}
		notificationID, err := a.recordAlertNotification(target.Name, target.Recipient, payload, "pending", ackToken, "")
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s save failed", target.Name))
			continue
		}
		err = sendAlertToChannel(payload, target.Name, target.Webhook, target.Recipient, ackToken, proxyConfig)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s send failed: %v", target.Name, err))
			_ = a.updateAlertNotificationStatus(notificationID, "failed", err.Error())
			continue
		}
		hadSuccess = true
		_ = a.updateAlertNotificationStatus(notificationID, "sent", "")
	}

	if !hadEnabled {
		return fmt.Errorf("all outbound channels are disabled")
	}
	if !hadSuccess && len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	if len(errs) > 0 {
		appLog.Warnw("some outbound notifications failed", "errors", strings.Join(errs, "; "))
	}
	return nil
}

func sendAlertToChannel(payload alertPayload, channel, webhookURL, recipient, ackToken string, proxyConfig gotifyConfig) error {
	if strings.TrimSpace(webhookURL) == "" {
		return fmt.Errorf("webhook url is empty")
	}
	body, err := json.Marshal(map[string]any{
		"channel":    channel,
		"title":      payload.Title,
		"message":    fmt.Sprintf("%s | Patient: %s | Chambre: %s | Lit: %s | Heure: %s | Origine: %s", payload.Reason, payload.Patient, payload.Room, payload.Bed, payload.TimeHM, payload.SourceUser),
		"recipient":  strings.TrimSpace(recipient),
		"reason":     payload.Reason,
		"patient":    payload.Patient,
		"room":       payload.Room,
		"bed":        payload.Bed,
		"timeHM":     payload.TimeHM,
		"sourceUser": payload.SourceUser,
		"ackToken":   ackToken,
	})
	if err != nil {
		return fmt.Errorf("marshal payload failed: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimSpace(webhookURL), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client, err := buildOutboundHTTPClient(proxyConfig.ProxyEnabled, proxyConfig.ProxyURL, proxyConfig.ProxyUsername, proxyConfig.ProxyPassword)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook call failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func (a *App) recordAlertNotification(channel, recipient string, payload alertPayload, status, ackToken, errorText string) (uint, error) {
	record := AlertNotification{
		Channel:   strings.TrimSpace(channel),
		Recipient: strings.TrimSpace(recipient),
		Title:     payload.Title,
		Message:   payload.Patient,
		Status:    strings.TrimSpace(status),
		AckToken:  strings.TrimSpace(ackToken),
		ErrorText: strings.TrimSpace(errorText),
	}
	if err := a.db.Create(&record).Error; err != nil {
		return 0, err
	}
	return record.ID, nil
}

func (a *App) updateAlertNotificationStatus(id uint, status, errorText string) error {
	updates := map[string]any{"status": strings.TrimSpace(status), "error_text": strings.TrimSpace(errorText)}
	return a.db.Model(&AlertNotification{}).Where("id = ?", id).Updates(updates).Error
}

func (a *App) acknowledgeAlertNotificationByID(id uint) error {
	var notification AlertNotification
	if err := a.db.Where("id = ?", id).Limit(1).Find(&notification).Error; err != nil {
		return err
	}
	if notification.ID == 0 {
		return fmt.Errorf("notification not found")
	}
	now := time.Now()
	return a.db.Model(&notification).Updates(map[string]any{"status": "acknowledged", "acknowledged_at": &now}).Error
}

func (a *App) acknowledgeAlertNotificationByToken(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token required")
	}
	var notification AlertNotification
	if err := a.db.Where("ack_token = ?", token).Order("id desc").Limit(1).Find(&notification).Error; err != nil {
		return err
	}
	if notification.ID == 0 {
		return fmt.Errorf("notification not found")
	}
	now := time.Now()
	return a.db.Model(&notification).Updates(map[string]any{"status": "acknowledged", "acknowledged_at": &now}).Error
}
