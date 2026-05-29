package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	settingGotifyEnabled         = "gotify.enabled"
	settingGotifyURL             = "gotify.url"
	settingGotifyToken           = "gotify.token"
	settingGotifyPriority        = "gotify.priority"
	settingUIBrandName           = "ui.brand_name"
	settingUIBrandLogo           = "ui.brand_logo_data_url"
	settingUILocale              = "ui.locale"
	settingAdminInitUsername     = "security.admin_init_username"
	settingAdminInitPassword     = "security.admin_init_password"
	settingForceSecureCookie     = "security.force_secure_cookie"
	settingTrustProxyHeaders     = "security.trust_proxy_headers"
	settingEnableHSTS            = "security.enable_hsts"
	settingHSTSMaxAge            = "security.hsts_max_age"
	settingHSTSIncludeSubdomains = "security.hsts_include_subdomains"
	settingHSTSPreload           = "security.hsts_preload"
	settingGotifyTokenEncKey     = "security.gotify_token_enc_key"
	settingTriageSLAMinutes      = "security.triage_sla_minutes"
	settingProxyEnabled          = "security.proxy_enabled"
	settingProxyURL              = "security.proxy_url"
	settingProxyUsername         = "security.proxy_username"
	settingProxyPassword         = "security.proxy_password"
	encryptedSecretPrefix        = "enc:v1:"
)

type securityConfigView struct {
	AdminInitUsername           string `json:"adminInitUsername"`
	AdminInitPasswordConfigured bool   `json:"adminInitPasswordConfigured"`
	ForceSecureCookie           bool   `json:"forceSecureCookie"`
	TrustProxyHeaders           bool   `json:"trustProxyHeaders"`
	EnableHSTS                  bool   `json:"enableHsts"`
	HSTSMaxAge                  int    `json:"hstsMaxAge"`
	HSTSIncludeSubdomains       bool   `json:"hstsIncludeSubdomains"`
	HSTSPreload                 bool   `json:"hstsPreload"`
	GotifyTokenEncKeyConfigured bool   `json:"gotifyTokenEncKeyConfigured"`
	TriageSLAMinutes            int    `json:"triageSlaMinutes"`
	ProxyEnabled                bool   `json:"proxyEnabled"`
	ProxyURL                    string `json:"proxyUrl"`
	ProxyUsername               string `json:"proxyUsername"`
	ProxyPasswordConfigured     bool   `json:"proxyPasswordConfigured"`
}

type securityConfigRequest struct {
	AdminInitUsername      string `json:"adminInitUsername"`
	AdminInitPassword      string `json:"adminInitPassword"`
	ForceSecureCookie      bool   `json:"forceSecureCookie"`
	TrustProxyHeaders      bool   `json:"trustProxyHeaders"`
	EnableHSTS             bool   `json:"enableHsts"`
	HSTSMaxAge             int    `json:"hstsMaxAge"`
	HSTSIncludeSubdomains  bool   `json:"hstsIncludeSubdomains"`
	HSTSPreload            bool   `json:"hstsPreload"`
	GotifyTokenEncKey      string `json:"gotifyTokenEncKey"`
	TriageSLAMinutes       int    `json:"triageSlaMinutes"`
	ProxyEnabled           bool   `json:"proxyEnabled"`
	ProxyURL               string `json:"proxyUrl"`
	ProxyUsername          string `json:"proxyUsername"`
	ProxyPassword          string `json:"proxyPassword"`
	ClearProxyPassword     bool   `json:"clearProxyPassword"`
	ClearAdminInitPassword bool   `json:"clearAdminInitPassword"`
	ClearGotifyTokenEncKey bool   `json:"clearGotifyTokenEncKey"`
}

type uiConfigView struct {
	AppName     string `json:"appName"`
	LogoDataURL string `json:"logoDataUrl"`
	Locale      string `json:"locale"`
}

type uiConfigRequest struct {
	AppName     string `json:"appName"`
	LogoDataURL string `json:"logoDataUrl"`
	Locale      string `json:"locale"`
	ClearLogo   bool   `json:"clearLogo"`
}

type gotifyTestRequest struct {
	Title   string `json:"title"`
	Message string `json:"message"`
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
		log.Printf("gotify token decrypt failed: %v", err)
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
		log.Printf("proxy password decrypt failed: %v", err)
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
	return securityConfigView{
		AdminInitUsername:           username,
		AdminInitPasswordConfigured: adminPwd != "",
		ForceSecureCookie:           a.getSettingBool(settingForceSecureCookie, envBool("FORCE_SECURE_COOKIE", false)),
		TrustProxyHeaders:           a.getSettingBool(settingTrustProxyHeaders, envBool("TRUST_PROXY_HEADERS", false)),
		EnableHSTS:                  a.getSettingBool(settingEnableHSTS, envBool("ENABLE_HSTS", false)),
		HSTSMaxAge:                  a.getSettingInt(settingHSTSMaxAge, envInt("HSTS_MAX_AGE", 31536000)),
		HSTSIncludeSubdomains:       a.getSettingBool(settingHSTSIncludeSubdomains, envBool("HSTS_INCLUDE_SUBDOMAINS", true)),
		HSTSPreload:                 a.getSettingBool(settingHSTSPreload, envBool("HSTS_PRELOAD", false)),
		GotifyTokenEncKeyConfigured: encKey != "",
		TriageSLAMinutes:            a.getSettingInt(settingTriageSLAMinutes, envInt("TRIAGE_SLA_MINUTES", 15)),
		ProxyEnabled:                a.getSettingBool(settingProxyEnabled, false),
		ProxyURL:                    proxyURL,
		ProxyUsername:               proxyUsername,
		ProxyPasswordConfigured:     proxyPassword != "",
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
	return uiConfigView{
		AppName:     name,
		LogoDataURL: logo,
		Locale:      locale,
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
