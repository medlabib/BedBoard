package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	settingGotifyEnabled                  = "gotify.enabled"
	settingGotifyURL                      = "gotify.url"
	settingGotifyToken                    = "gotify.token"
	settingGotifyPriority                 = "gotify.priority"
	settingUIBrandName                    = "ui.brand_name"
	settingUIBrandLogo                    = "ui.brand_logo_data_url"
	settingUILocale                       = "ui.locale"
	settingUIPatientViewIdentityMode      = "ui.patient_view_identity_mode"
	settingAdminInitUsername              = "security.admin_init_username"
	settingAdminInitPassword              = "security.admin_init_password"
	settingForceSecureCookie              = "security.force_secure_cookie"
	settingTrustProxyHeaders              = "security.trust_proxy_headers"
	settingEnableHSTS                     = "security.enable_hsts"
	settingHSTSMaxAge                     = "security.hsts_max_age"
	settingHSTSIncludeSubdomains          = "security.hsts_include_subdomains"
	settingHSTSPreload                    = "security.hsts_preload"
	settingGotifyTokenEncKey              = "security.gotify_token_enc_key"
	settingTriageSLAMinutes               = "security.triage_sla_minutes"
	settingProxyEnabled                   = "security.proxy_enabled"
	settingProxyURL                       = "security.proxy_url"
	settingProxyUsername                  = "security.proxy_username"
	settingProxyPassword                  = "security.proxy_password"
	settingAlertCallbackSignatureRequired = "security.alert_callback_signature_required"
	settingAlertCallbackSecret            = "security.alert_callback_secret"
	settingAlertCallbackIPAllowlist       = "security.alert_callback_ip_allowlist"
	settingSMSEnabled                     = "integrations.sms.enabled"
	settingSMSWebhookURL                  = "integrations.sms.webhook_url"
	settingSMSRecipient                   = "integrations.sms.recipient"
	settingWhatsAppEnabled                = "integrations.whatsapp.enabled"
	settingWhatsAppWebhookURL             = "integrations.whatsapp.webhook_url"
	settingWhatsAppRecipient              = "integrations.whatsapp.recipient"
	encryptedSecretPrefix                 = "enc:v1:"
)

type securityConfigView struct {
	AdminInitUsername              string `json:"adminInitUsername"`
	AdminInitPasswordConfigured    bool   `json:"adminInitPasswordConfigured"`
	ForceSecureCookie              bool   `json:"forceSecureCookie"`
	TrustProxyHeaders              bool   `json:"trustProxyHeaders"`
	EnableHSTS                     bool   `json:"enableHsts"`
	HSTSMaxAge                     int    `json:"hstsMaxAge"`
	HSTSIncludeSubdomains          bool   `json:"hstsIncludeSubdomains"`
	HSTSPreload                    bool   `json:"hstsPreload"`
	GotifyTokenEncKeyConfigured    bool   `json:"gotifyTokenEncKeyConfigured"`
	TriageSLAMinutes               int    `json:"triageSlaMinutes"`
	ProxyEnabled                   bool   `json:"proxyEnabled"`
	ProxyURL                       string `json:"proxyUrl"`
	ProxyUsername                  string `json:"proxyUsername"`
	ProxyPasswordConfigured        bool   `json:"proxyPasswordConfigured"`
	AlertCallbackSignatureRequired bool   `json:"alertCallbackSignatureRequired"`
	AlertCallbackSecretConfigured  bool   `json:"alertCallbackSecretConfigured"`
	AlertCallbackIPAllowlist       string `json:"alertCallbackIpAllowlist"`
}

type securityConfigRequest struct {
	AdminInitUsername              string `json:"adminInitUsername"`
	AdminInitPassword              string `json:"adminInitPassword"`
	ForceSecureCookie              bool   `json:"forceSecureCookie"`
	TrustProxyHeaders              bool   `json:"trustProxyHeaders"`
	EnableHSTS                     bool   `json:"enableHsts"`
	HSTSMaxAge                     int    `json:"hstsMaxAge"`
	HSTSIncludeSubdomains          bool   `json:"hstsIncludeSubdomains"`
	HSTSPreload                    bool   `json:"hstsPreload"`
	GotifyTokenEncKey              string `json:"gotifyTokenEncKey"`
	TriageSLAMinutes               int    `json:"triageSlaMinutes"`
	ProxyEnabled                   bool   `json:"proxyEnabled"`
	ProxyURL                       string `json:"proxyUrl"`
	ProxyUsername                  string `json:"proxyUsername"`
	ProxyPassword                  string `json:"proxyPassword"`
	AlertCallbackSignatureRequired bool   `json:"alertCallbackSignatureRequired"`
	AlertCallbackSecret            string `json:"alertCallbackSecret"`
	AlertCallbackIPAllowlist       string `json:"alertCallbackIpAllowlist"`
	ClearProxyPassword             bool   `json:"clearProxyPassword"`
	ClearAlertCallbackSecret       bool   `json:"clearAlertCallbackSecret"`
	ClearAdminInitPassword         bool   `json:"clearAdminInitPassword"`
	ClearGotifyTokenEncKey         bool   `json:"clearGotifyTokenEncKey"`
}

type uiConfigView struct {
	AppName                 string `json:"appName"`
	LogoDataURL             string `json:"logoDataUrl"`
	Locale                  string `json:"locale"`
	PatientViewIdentityMode string `json:"patientViewIdentityMode"`
}

type uiConfigRequest struct {
	AppName                 string `json:"appName"`
	LogoDataURL             string `json:"logoDataUrl"`
	Locale                  string `json:"locale"`
	PatientViewIdentityMode string `json:"patientViewIdentityMode"`
	ClearLogo               bool   `json:"clearLogo"`
}

type gotifyTestRequest struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

type outboundAlertChannel struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhookUrl"`
	Recipient  string `json:"recipient"`
}

type alertChannelsSettingsView struct {
	SMS      outboundAlertChannel `json:"sms"`
	WhatsApp outboundAlertChannel `json:"whatsapp"`
}

type alertChannelsSettingsRequest struct {
	SMS      outboundAlertChannel `json:"sms"`
	WhatsApp outboundAlertChannel `json:"whatsapp"`
}

type alertNotificationView struct {
	ID             uint       `json:"id"`
	Channel        string     `json:"channel"`
	Recipient      string     `json:"recipient"`
	Title          string     `json:"title"`
	Message        string     `json:"message"`
	Status         string     `json:"status"`
	ErrorText      string     `json:"errorText"`
	CreatedAt      time.Time  `json:"createdAt"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt"`
}

type alertChannelsTestRequest struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

type alertNotificationAckRequest struct {
	ID uint `json:"id"`
}

type alertTokenAckRequest struct {
	Token string `json:"token"`
}

func normalizeUILocale(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "en":
		return "en"
	case "ar":
		return "ar"
	default:
		return "fr"
	}
}

func normalizeUIPatientViewIdentityMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "number":
		return "number"
	default:
		return "name"
	}
}

func validateGotifyURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid gotify url")
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("gotify url must use http or https")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("gotify url host is required")
	}
	return nil
}

func validateWebhookURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid webhook url")
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("webhook url must use http or https")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("webhook url host is required")
	}
	return nil
}

func sanitizeRecipient(value string) string {
	cleaned := strings.TrimSpace(value)
	if len(cleaned) > 120 {
		return cleaned[:120]
	}
	return cleaned
}

func validateProxyURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid proxy url")
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("proxy url must use http or https")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("proxy url host is required")
	}
	return nil
}

func normalizeIPAllowlist(raw string) string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	})
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		clean = append(clean, trimmed)
	}
	return strings.Join(clean, ",")
}

func validateIPAllowlist(raw string) error {
	normalized := normalizeIPAllowlist(raw)
	if normalized == "" {
		return nil
	}
	for _, candidate := range strings.Split(normalized, ",") {
		entry := strings.TrimSpace(candidate)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			if _, err := netip.ParsePrefix(entry); err != nil {
				return fmt.Errorf("invalid allowlist CIDR: %s", entry)
			}
			continue
		}
		if ip := net.ParseIP(entry); ip == nil {
			return fmt.Errorf("invalid allowlist IP: %s", entry)
		}
	}
	return nil
}

func ipAllowedByAllowlist(clientIP, allowlist string) bool {
	normalized := normalizeIPAllowlist(allowlist)
	if normalized == "" {
		return true
	}
	ipAddr, err := netip.ParseAddr(strings.TrimSpace(clientIP))
	if err != nil {
		return false
	}
	for _, candidate := range strings.Split(normalized, ",") {
		entry := strings.TrimSpace(candidate)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			prefix, err := netip.ParsePrefix(entry)
			if err == nil && prefix.Contains(ipAddr) {
				return true
			}
			continue
		}
		allowedIP, err := netip.ParseAddr(entry)
		if err == nil && allowedIP == ipAddr {
			return true
		}
	}
	return false
}

func (a *App) extractClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if a.getSettingBool(settingTrustProxyHeaders, envBool("TRUST_PROXY_HEADERS", false)) {
		if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return strings.TrimSpace(host)
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func parseSignatureHeader(raw string) string {
	signature := strings.TrimSpace(raw)
	signature = strings.TrimPrefix(signature, "sha256=")
	return strings.TrimSpace(signature)
}

func computeCallbackHMAC(secret, timestamp string, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(timestamp))
	_, _ = h.Write([]byte("."))
	_, _ = h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func (a *App) verifyAlertAckCallback(r *http.Request, rawBody []byte) error {
	allowlist := strings.TrimSpace(firstNonEmpty(a.getSettingValue(settingAlertCallbackIPAllowlist), os.Getenv("ALERT_CALLBACK_IP_ALLOWLIST")))
	clientIP := a.extractClientIP(r)
	if !ipAllowedByAllowlist(clientIP, allowlist) {
		return fmt.Errorf("callback source ip not allowed")
	}

	signatureRequired := a.getSettingBool(settingAlertCallbackSignatureRequired, envBool("ALERT_CALLBACK_SIGNATURE_REQUIRED", true))
	if !signatureRequired {
		return nil
	}
	secretValue := strings.TrimSpace(a.getSettingValue(settingAlertCallbackSecret))
	secret, err := a.decryptSecretValue(secretValue)
	if err != nil {
		appLog.Warnw("alert callback secret decrypt failed", "error", err)
		secret = ""
	}
	if strings.TrimSpace(secret) == "" {
		secret = strings.TrimSpace(os.Getenv("ALERT_CALLBACK_SECRET"))
	}
	if strings.TrimSpace(secret) == "" {
		return fmt.Errorf("alert callback secret not configured")
	}

	timestampHeader := strings.TrimSpace(r.Header.Get("X-BedBoard-Timestamp"))
	if timestampHeader == "" {
		return fmt.Errorf("missing callback timestamp")
	}
	timestampValue, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid callback timestamp")
	}
	now := time.Now().Unix()
	if timestampValue < now-300 || timestampValue > now+300 {
		return fmt.Errorf("callback timestamp out of range")
	}
	signatureHeader := parseSignatureHeader(r.Header.Get("X-BedBoard-Signature"))
	if signatureHeader == "" {
		return fmt.Errorf("missing callback signature")
	}
	if _, err := hex.DecodeString(signatureHeader); err != nil {
		return fmt.Errorf("invalid callback signature encoding")
	}
	expectedSignature := computeCallbackHMAC(secret, timestampHeader, rawBody)
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(signatureHeader)), []byte(strings.ToLower(expectedSignature))) != 1 {
		return fmt.Errorf("invalid callback signature")
	}
	return nil
}

func (a *App) readTokenEncryptionKey() ([]byte, error) {
	raw := strings.TrimSpace(a.getSettingValue(settingGotifyTokenEncKey))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("GOTIFY_TOKEN_ENC_KEY"))
	}
	if raw == "" {
		return nil, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid GOTIFY_TOKEN_ENC_KEY encoding")
		}
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("GOTIFY_TOKEN_ENC_KEY must decode to 32 bytes")
	}
	return decoded, nil
}

func (a *App) getSettingValue(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	var setting AppSetting
	result := a.db.Where("key = ?", key).Limit(1).Find(&setting)
	if result.Error != nil || result.RowsAffected == 0 {
		return ""
	}
	return setting.Value
}

func (a *App) upsertSettingValue(key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	var setting AppSetting
	result := a.db.Where("key = ?", key).Limit(1).Find(&setting)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		setting.Value = value
		return a.db.Save(&setting).Error
	}
	setting = AppSetting{Key: key, Value: value}
	return a.db.Create(&setting).Error
}

func (a *App) getGotifyConfig() gotifyConfig {
	url := strings.TrimSpace(a.getSettingValue(settingGotifyURL))
	tokenValue := strings.TrimSpace(a.getSettingValue(settingGotifyToken))
	token, err := a.decryptSecretValue(tokenValue)
	if err != nil {
		appLog.Warnw("gotify token decrypt failed", "error", err)
		token = ""
	}
	priority := 8
	if parsed := strings.TrimSpace(a.getSettingValue(settingGotifyPriority)); parsed != "" {
		if value, err := strconv.Atoi(parsed); err == nil {
			priority = value
		}
	}
	enabledRaw := strings.TrimSpace(strings.ToLower(a.getSettingValue(settingGotifyEnabled)))
	enabled := enabledRaw == "1" || enabledRaw == "true" || enabledRaw == "yes" || enabledRaw == "on"
	if !enabled && enabledRaw == "" {
		urlEnv := strings.TrimSpace(os.Getenv("GOTIFY_URL"))
		tokenEnv := strings.TrimSpace(os.Getenv("GOTIFY_TOKEN"))
		if url == "" {
			url = urlEnv
		}
		if token == "" {
			token = tokenEnv
		}
		if priority == 8 {
			priority = envInt("GOTIFY_PRIORITY", 8)
		}
		enabled = url != ""
	}
	proxyURL := strings.TrimSpace(a.getSettingValue(settingProxyURL))
	proxyUsername := strings.TrimSpace(a.getSettingValue(settingProxyUsername))
	proxyPasswordValue := strings.TrimSpace(a.getSettingValue(settingProxyPassword))
	proxyPassword, err := a.decryptSecretValue(proxyPasswordValue)
	if err != nil {
		appLog.Warnw("proxy password decrypt failed", "error", err)
		proxyPassword = ""
	}
	proxyEnabled := a.getSettingBool(settingProxyEnabled, false)
	return gotifyConfig{
		Enabled:       enabled,
		URL:           url,
		Token:         token,
		Priority:      priority,
		ProxyEnabled:  proxyEnabled,
		ProxyURL:      proxyURL,
		ProxyUsername: proxyUsername,
		ProxyPassword: proxyPassword,
	}
}

func (a *App) handleGotifySettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := a.getGotifyConfig()
		writeJSON(w, http.StatusOK, gotifySettingsView{
			Enabled:         cfg.Enabled,
			URL:             cfg.URL,
			Priority:        cfg.Priority,
			TokenConfigured: strings.TrimSpace(cfg.Token) != "",
		})
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var req gotifySettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		url := strings.TrimSpace(req.URL)
		if req.Enabled && url == "" {
			http.Error(w, "gotify url required when enabled", http.StatusBadRequest)
			return
		}
		if err := validateGotifyURL(url); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		hasExistingToken := strings.TrimSpace(a.getGotifyConfig().Token) != ""
		incomingToken := strings.TrimSpace(req.Token)
		if req.Enabled {
			if req.ClearToken && incomingToken == "" {
				http.Error(w, "gotify token required when enabled", http.StatusBadRequest)
				return
			}
			if !hasExistingToken && incomingToken == "" {
				http.Error(w, "gotify token required when enabled", http.StatusBadRequest)
				return
			}
		}
		priority := req.Priority
		if priority == 0 {
			priority = 8
		}
		if priority < 1 || priority > 10 {
			http.Error(w, "priority must be 1..10", http.StatusBadRequest)
			return
		}
		if err := a.upsertSettingValue(settingGotifyEnabled, strconv.FormatBool(req.Enabled)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingGotifyURL, url); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingGotifyPriority, strconv.Itoa(priority)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if req.ClearToken {
			if err := a.upsertSettingValue(settingGotifyToken, ""); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
		} else if incomingToken != "" {
			encryptedToken, err := a.encryptSecretValue(incomingToken)
			if err != nil {
				http.Error(w, "token encryption failed", http.StatusInternalServerError)
				return
			}
			if err := a.upsertSettingValue(settingGotifyToken, encryptedToken); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
		}
		cfg := a.getGotifyConfig()
		a.broadcastEvent("system.settings", map[string]any{"scope": "gotify"})
		writeJSON(w, http.StatusOK, gotifySettingsView{
			Enabled:         cfg.Enabled,
			URL:             cfg.URL,
			Priority:        cfg.Priority,
			TokenConfigured: strings.TrimSpace(cfg.Token) != "",
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) handleGotifyTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req gotifyTestRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "BedBoard Test"
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		message = "This is a test notification from BedBoard admin settings."
	}
	payload := alertPayload{
		Title:      title,
		Reason:     "admin_test",
		Patient:    message,
		Room:       "N/A",
		Bed:        "N/A",
		SourceUser: "admin",
		TimeHM:     time.Now().Format("15:04"),
	}
	if err := sendGotifyAlertPayload(payload, a.getGotifyConfig()); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) getAlertChannelsConfig() alertChannelsSettingsView {
	return alertChannelsSettingsView{
		SMS: outboundAlertChannel{
			Enabled:    a.getSettingBool(settingSMSEnabled, envBool("SMS_ALERTS_ENABLED", false)),
			WebhookURL: strings.TrimSpace(firstNonEmpty(a.getSettingValue(settingSMSWebhookURL), os.Getenv("SMS_ALERTS_WEBHOOK_URL"))),
			Recipient:  sanitizeRecipient(firstNonEmpty(a.getSettingValue(settingSMSRecipient), os.Getenv("SMS_ALERTS_RECIPIENT"))),
		},
		WhatsApp: outboundAlertChannel{
			Enabled:    a.getSettingBool(settingWhatsAppEnabled, envBool("WHATSAPP_ALERTS_ENABLED", false)),
			WebhookURL: strings.TrimSpace(firstNonEmpty(a.getSettingValue(settingWhatsAppWebhookURL), os.Getenv("WHATSAPP_ALERTS_WEBHOOK_URL"))),
			Recipient:  sanitizeRecipient(firstNonEmpty(a.getSettingValue(settingWhatsAppRecipient), os.Getenv("WHATSAPP_ALERTS_RECIPIENT"))),
		},
	}
}

func firstNonEmpty(primary, fallback string) string {
	trimmed := strings.TrimSpace(primary)
	if trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(fallback)
}

func (a *App) handleAlertChannelsSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.getAlertChannelsConfig())
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var req alertChannelsSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		req.SMS.WebhookURL = strings.TrimSpace(req.SMS.WebhookURL)
		req.WhatsApp.WebhookURL = strings.TrimSpace(req.WhatsApp.WebhookURL)
		req.SMS.Recipient = sanitizeRecipient(req.SMS.Recipient)
		req.WhatsApp.Recipient = sanitizeRecipient(req.WhatsApp.Recipient)
		if req.SMS.Enabled && req.SMS.WebhookURL == "" {
			http.Error(w, "sms webhook url required when enabled", http.StatusBadRequest)
			return
		}
		if req.WhatsApp.Enabled && req.WhatsApp.WebhookURL == "" {
			http.Error(w, "whatsapp webhook url required when enabled", http.StatusBadRequest)
			return
		}
		if err := validateWebhookURL(req.SMS.WebhookURL); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := validateWebhookURL(req.WhatsApp.WebhookURL); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := a.upsertSettingValue(settingSMSEnabled, strconv.FormatBool(req.SMS.Enabled)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingSMSWebhookURL, req.SMS.WebhookURL); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingSMSRecipient, req.SMS.Recipient); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingWhatsAppEnabled, strconv.FormatBool(req.WhatsApp.Enabled)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingWhatsAppWebhookURL, req.WhatsApp.WebhookURL); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingWhatsAppRecipient, req.WhatsApp.Recipient); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		a.broadcastEvent("system.settings", map[string]any{"scope": "integrations.alert_channels"})
		writeJSON(w, http.StatusOK, a.getAlertChannelsConfig())
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) handleAlertChannelsTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req alertChannelsTestRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "BedBoard Channel Test"
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		message = "This is a test outbound message from BedBoard."
	}
	payload := alertPayload{
		Title:      title,
		Reason:     "admin_test",
		Patient:    message,
		Room:       "N/A",
		Bed:        "N/A",
		SourceUser: "admin",
		TimeHM:     time.Now().Format("15:04"),
	}
	if err := a.sendAlertChannels(payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleAlertNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	notifications := make([]AlertNotification, 0)
	if err := a.db.Order("created_at desc").Limit(200).Find(&notifications).Error; err != nil {
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}
	views := make([]alertNotificationView, 0, len(notifications))
	for _, item := range notifications {
		views = append(views, alertNotificationView{
			ID:             item.ID,
			Channel:        item.Channel,
			Recipient:      item.Recipient,
			Title:          item.Title,
			Message:        item.Message,
			Status:         item.Status,
			ErrorText:      item.ErrorText,
			CreatedAt:      item.CreatedAt,
			AcknowledgedAt: item.AcknowledgedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": views})
}

func (a *App) handleAlertNotificationAck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req alertNotificationAckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.ID == 0 {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if err := a.acknowledgeAlertNotificationByID(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleAlertAckByToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := a.verifyAlertAckCallback(r, rawBody); err != nil {
		appLog.Warnw("alert callback rejected", "error", err, "remote", a.extractClientIP(r))
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	var req alertTokenAckRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}
	if err := a.acknowledgeAlertNotificationByToken(token); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) encryptSecretValue(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	key, err := a.readTokenEncryptionKey()
	if err != nil {
		return "", err
	}
	if key == nil {
		return value, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	cipherText := gcm.Seal(nil, nonce, []byte(value), nil)
	packed := append(nonce, cipherText...)
	return encryptedSecretPrefix + base64.RawStdEncoding.EncodeToString(packed), nil
}

func (a *App) decryptSecretValue(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if !strings.HasPrefix(trimmed, encryptedSecretPrefix) {
		return trimmed, nil
	}
	key, err := a.readTokenEncryptionKey()
	if err != nil {
		return "", err
	}
	if key == nil {
		return "", fmt.Errorf("encrypted value found but GOTIFY_TOKEN_ENC_KEY is missing")
	}
	rawCipher, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(trimmed, encryptedSecretPrefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(rawCipher) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted value too short")
	}
	nonce, cipherText := rawCipher[:gcm.NonceSize()], rawCipher[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (a *App) getSettingBool(key string, def bool) bool {
	raw := strings.TrimSpace(strings.ToLower(a.getSettingValue(key)))
	if raw == "" {
		return def
	}
	if raw == "1" || raw == "true" || raw == "yes" || raw == "on" {
		return true
	}
	if raw == "0" || raw == "false" || raw == "no" || raw == "off" {
		return false
	}
	return def
}

func (a *App) getSettingInt(key string, def int) int {
	raw := strings.TrimSpace(a.getSettingValue(key))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

func (a *App) getSecurityConfigView() securityConfigView {
	username := strings.TrimSpace(a.getSettingValue(settingAdminInitUsername))
	if username == "" {
		username = strings.TrimSpace(os.Getenv("ADMIN_INIT_USERNAME"))
	}
	if username == "" {
		username = defaultUsername
	}
	adminPwd := strings.TrimSpace(a.getSettingValue(settingAdminInitPassword))
	if adminPwd == "" {
		adminPwd = strings.TrimSpace(os.Getenv("ADMIN_INIT_PASSWORD"))
	}
	encKey := strings.TrimSpace(a.getSettingValue(settingGotifyTokenEncKey))
	if encKey == "" {
		encKey = strings.TrimSpace(os.Getenv("GOTIFY_TOKEN_ENC_KEY"))
	}
	proxyURL := strings.TrimSpace(a.getSettingValue(settingProxyURL))
	proxyUsername := strings.TrimSpace(a.getSettingValue(settingProxyUsername))
	proxyPassword := strings.TrimSpace(a.getSettingValue(settingProxyPassword))
	callbackSecretValue := strings.TrimSpace(a.getSettingValue(settingAlertCallbackSecret))
	callbackSecret, err := a.decryptSecretValue(callbackSecretValue)
	if err != nil {
		appLog.Warnw("alert callback secret decrypt failed", "error", err)
		callbackSecret = ""
	}
	if callbackSecret == "" {
		callbackSecret = strings.TrimSpace(os.Getenv("ALERT_CALLBACK_SECRET"))
	}
	callbackAllowlist := normalizeIPAllowlist(firstNonEmpty(a.getSettingValue(settingAlertCallbackIPAllowlist), os.Getenv("ALERT_CALLBACK_IP_ALLOWLIST")))
	return securityConfigView{
		AdminInitUsername:              username,
		AdminInitPasswordConfigured:    adminPwd != "",
		ForceSecureCookie:              a.getSettingBool(settingForceSecureCookie, envBool("FORCE_SECURE_COOKIE", false)),
		TrustProxyHeaders:              a.getSettingBool(settingTrustProxyHeaders, envBool("TRUST_PROXY_HEADERS", false)),
		EnableHSTS:                     a.getSettingBool(settingEnableHSTS, envBool("ENABLE_HSTS", false)),
		HSTSMaxAge:                     a.getSettingInt(settingHSTSMaxAge, envInt("HSTS_MAX_AGE", 31536000)),
		HSTSIncludeSubdomains:          a.getSettingBool(settingHSTSIncludeSubdomains, envBool("HSTS_INCLUDE_SUBDOMAINS", true)),
		HSTSPreload:                    a.getSettingBool(settingHSTSPreload, envBool("HSTS_PRELOAD", false)),
		GotifyTokenEncKeyConfigured:    encKey != "",
		TriageSLAMinutes:               a.getSettingInt(settingTriageSLAMinutes, envInt("TRIAGE_SLA_MINUTES", 15)),
		ProxyEnabled:                   a.getSettingBool(settingProxyEnabled, false),
		ProxyURL:                       proxyURL,
		ProxyUsername:                  proxyUsername,
		ProxyPasswordConfigured:        proxyPassword != "",
		AlertCallbackSignatureRequired: a.getSettingBool(settingAlertCallbackSignatureRequired, envBool("ALERT_CALLBACK_SIGNATURE_REQUIRED", true)),
		AlertCallbackSecretConfigured:  strings.TrimSpace(callbackSecret) != "",
		AlertCallbackIPAllowlist:       callbackAllowlist,
	}
}

func (a *App) handleSecurityConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.getSecurityConfigView())
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var req securityConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		username := strings.TrimSpace(req.AdminInitUsername)
		if username == "" {
			http.Error(w, "admin init username required", http.StatusBadRequest)
			return
		}
		if req.HSTSMaxAge < 0 {
			http.Error(w, "hsts max age must be >= 0", http.StatusBadRequest)
			return
		}
		if req.TriageSLAMinutes == 0 {
			req.TriageSLAMinutes = a.getSettingInt(settingTriageSLAMinutes, envInt("TRIAGE_SLA_MINUTES", 15))
		}
		if req.TriageSLAMinutes < 1 || req.TriageSLAMinutes > 240 {
			http.Error(w, "triage SLA minutes must be 1..240", http.StatusBadRequest)
			return
		}
		proxyURL := strings.TrimSpace(req.ProxyURL)
		if req.ProxyEnabled && proxyURL == "" {
			http.Error(w, "proxy url required when proxy is enabled", http.StatusBadRequest)
			return
		}
		if err := validateProxyURL(proxyURL); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		callbackAllowlist := normalizeIPAllowlist(req.AlertCallbackIPAllowlist)
		if err := validateIPAllowlist(callbackAllowlist); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := a.upsertSettingValue(settingAdminInitUsername, username); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if req.ClearAdminInitPassword {
			if err := a.upsertSettingValue(settingAdminInitPassword, ""); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
		} else if strings.TrimSpace(req.AdminInitPassword) != "" {
			if err := a.upsertSettingValue(settingAdminInitPassword, strings.TrimSpace(req.AdminInitPassword)); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
		}
		if err := a.upsertSettingValue(settingForceSecureCookie, strconv.FormatBool(req.ForceSecureCookie)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingTrustProxyHeaders, strconv.FormatBool(req.TrustProxyHeaders)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingEnableHSTS, strconv.FormatBool(req.EnableHSTS)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingHSTSMaxAge, strconv.Itoa(req.HSTSMaxAge)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingHSTSIncludeSubdomains, strconv.FormatBool(req.HSTSIncludeSubdomains)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingHSTSPreload, strconv.FormatBool(req.HSTSPreload)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingTriageSLAMinutes, strconv.Itoa(req.TriageSLAMinutes)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingProxyEnabled, strconv.FormatBool(req.ProxyEnabled)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingProxyURL, proxyURL); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingProxyUsername, strings.TrimSpace(req.ProxyUsername)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if req.ClearProxyPassword {
			if err := a.upsertSettingValue(settingProxyPassword, ""); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
		} else if strings.TrimSpace(req.ProxyPassword) != "" {
			encryptedPassword, err := a.encryptSecretValue(strings.TrimSpace(req.ProxyPassword))
			if err != nil {
				http.Error(w, "proxy password encryption failed", http.StatusInternalServerError)
				return
			}
			if err := a.upsertSettingValue(settingProxyPassword, encryptedPassword); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
		}
		if err := a.upsertSettingValue(settingAlertCallbackSignatureRequired, strconv.FormatBool(req.AlertCallbackSignatureRequired)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingAlertCallbackIPAllowlist, callbackAllowlist); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if req.ClearAlertCallbackSecret {
			if err := a.upsertSettingValue(settingAlertCallbackSecret, ""); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
		} else if strings.TrimSpace(req.AlertCallbackSecret) != "" {
			encryptedCallbackSecret, err := a.encryptSecretValue(strings.TrimSpace(req.AlertCallbackSecret))
			if err != nil {
				http.Error(w, "callback secret encryption failed", http.StatusInternalServerError)
				return
			}
			if err := a.upsertSettingValue(settingAlertCallbackSecret, encryptedCallbackSecret); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
		}
		if req.ClearGotifyTokenEncKey {
			if err := a.upsertSettingValue(settingGotifyTokenEncKey, ""); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
		} else if strings.TrimSpace(req.GotifyTokenEncKey) != "" {
			if _, err := base64.StdEncoding.DecodeString(strings.TrimSpace(req.GotifyTokenEncKey)); err != nil {
				if _, errRaw := base64.RawStdEncoding.DecodeString(strings.TrimSpace(req.GotifyTokenEncKey)); errRaw != nil {
					http.Error(w, "invalid gotify token encryption key encoding", http.StatusBadRequest)
					return
				}
			}
			if err := a.upsertSettingValue(settingGotifyTokenEncKey, strings.TrimSpace(req.GotifyTokenEncKey)); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
		}
		a.broadcastEvent("system.settings", map[string]any{"scope": "security"})
		writeJSON(w, http.StatusOK, a.getSecurityConfigView())
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) getUIConfigView() uiConfigView {
	name := strings.TrimSpace(a.getSettingValue(settingUIBrandName))
	if name == "" {
		name = "BedBoard"
	}
	logo := strings.TrimSpace(a.getSettingValue(settingUIBrandLogo))
	locale := normalizeUILocale(a.getSettingValue(settingUILocale))
	identityMode := normalizeUIPatientViewIdentityMode(a.getSettingValue(settingUIPatientViewIdentityMode))
	return uiConfigView{
		AppName:                 name,
		LogoDataURL:             logo,
		Locale:                  locale,
		PatientViewIdentityMode: identityMode,
	}
}

func (a *App) handleUIConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.getUIConfigView())
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		var req uiConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(req.AppName)
		if name == "" {
			http.Error(w, "app name required", http.StatusBadRequest)
			return
		}
		if len(name) > 80 {
			http.Error(w, "app name too long", http.StatusBadRequest)
			return
		}
		if err := a.upsertSettingValue(settingUIBrandName, name); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if req.ClearLogo {
			if err := a.upsertSettingValue(settingUIBrandLogo, ""); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
		} else {
			logo := strings.TrimSpace(req.LogoDataURL)
			if logo != "" {
				if !strings.HasPrefix(strings.ToLower(logo), "data:image/") || !strings.Contains(logo, ";base64,") {
					http.Error(w, "invalid logo data url", http.StatusBadRequest)
					return
				}
				if len(logo) > 750000 {
					http.Error(w, "logo too large", http.StatusBadRequest)
					return
				}
				if err := a.upsertSettingValue(settingUIBrandLogo, logo); err != nil {
					http.Error(w, "save failed", http.StatusInternalServerError)
					return
				}
			}
		}
		if err := a.upsertSettingValue(settingUILocale, normalizeUILocale(req.Locale)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if err := a.upsertSettingValue(settingUIPatientViewIdentityMode, normalizeUIPatientViewIdentityMode(req.PatientViewIdentityMode)); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		a.broadcastEvent("system.settings", map[string]any{"scope": "ui"})
		writeJSON(w, http.StatusOK, a.getUIConfigView())
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) handlePublicUIConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, a.getUIConfigView())
}
