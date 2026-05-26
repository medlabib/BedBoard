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
	"os"
	"strconv"
	"strings"
)

const (
	settingGotifyEnabled  = "gotify.enabled"
	settingGotifyURL      = "gotify.url"
	settingGotifyToken    = "gotify.token"
	settingGotifyPriority = "gotify.priority"
	encryptedSecretPrefix = "enc:v1:"
)

func readTokenEncryptionKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("GOTIFY_TOKEN_ENC_KEY"))
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

func encryptSecretValue(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	key, err := readTokenEncryptionKey()
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

func decryptSecretValue(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if !strings.HasPrefix(trimmed, encryptedSecretPrefix) {
		return trimmed, nil
	}
	key, err := readTokenEncryptionKey()
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

func (a *App) getSettingValue(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	var setting AppSetting
	if err := a.db.Where("key = ?", key).First(&setting).Error; err != nil {
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
	if err := a.db.Where("key = ?", key).First(&setting).Error; err == nil {
		setting.Value = value
		return a.db.Save(&setting).Error
	}
	setting = AppSetting{Key: key, Value: value}
	return a.db.Create(&setting).Error
}

func (a *App) getGotifyConfig() gotifyConfig {
	url := strings.TrimSpace(a.getSettingValue(settingGotifyURL))
	tokenValue := strings.TrimSpace(a.getSettingValue(settingGotifyToken))
	token, err := decryptSecretValue(tokenValue)
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
	return gotifyConfig{Enabled: enabled, URL: url, Token: token, Priority: priority}
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
		} else if strings.TrimSpace(req.Token) != "" {
			encryptedToken, err := encryptSecretValue(strings.TrimSpace(req.Token))
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
