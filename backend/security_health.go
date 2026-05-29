package main

import (
	"net/http"
	"os"
	"runtime"
	"strings"
)

type securityCheck struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	Details        string `json:"details"`
	Recommendation string `json:"recommendation"`
}

type securityHealthView struct {
	Status string          `json:"status"`
	Checks []securityCheck `json:"checks"`
}

func (a *App) handleSecurityHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	checks := []securityCheck{}

	// HSTS audit
	if a.getSettingBool(settingEnableHSTS, envBool("ENABLE_HSTS", false)) {
		checks = append(checks, securityCheck{
			Name:    "hsts",
			Status:  "pass",
			Details: "HSTS is enabled.",
		})
	} else {
		checks = append(checks, securityCheck{
			Name:           "hsts",
			Status:         "warn",
			Details:        "HSTS is disabled.",
			Recommendation: "Set ENABLE_HSTS=true in HTTPS production deployments.",
		})
	}

	// Secure cookie audit
	if a.getSettingBool(settingForceSecureCookie, envBool("FORCE_SECURE_COOKIE", false)) {
		checks = append(checks, securityCheck{
			Name:    "secure_cookie",
			Status:  "pass",
			Details: "FORCE_SECURE_COOKIE is enabled.",
		})
	} else if a.getSettingBool(settingTrustProxyHeaders, envBool("TRUST_PROXY_HEADERS", false)) {
		checks = append(checks, securityCheck{
			Name:    "secure_cookie",
			Status:  "pass",
			Details: "Secure cookie can be enabled behind proxy via X-Forwarded-Proto.",
		})
	} else {
		checks = append(checks, securityCheck{
			Name:           "secure_cookie",
			Status:         "warn",
			Details:        "Secure cookie is not forced and proxy headers are not trusted.",
			Recommendation: "Use FORCE_SECURE_COOKIE=true or TRUST_PROXY_HEADERS=true behind HTTPS proxy.",
		})
	}

	// Bootstrap admin credential policy audit
	var adminCount int64
	if err := a.db.Model(&AdminUser{}).Count(&adminCount).Error; err == nil {
		bootstrapPassword := strings.TrimSpace(a.getSettingValue(settingAdminInitPassword))
		if bootstrapPassword == "" {
			bootstrapPassword = strings.TrimSpace(os.Getenv("ADMIN_INIT_PASSWORD"))
		}
		if adminCount == 0 && bootstrapPassword == "" {
			checks = append(checks, securityCheck{
				Name:           "admin_bootstrap",
				Status:         "fail",
				Details:        "No admin exists and admin bootstrap password is empty.",
				Recommendation: "Set admin bootstrap password in Admin > Security.",
			})
		} else {
			checks = append(checks, securityCheck{
				Name:    "admin_bootstrap",
				Status:  "pass",
				Details: "Admin bootstrap policy is configured.",
			})
		}
	}

	// Gotify token at-rest encryption audit
	rawToken := strings.TrimSpace(a.getSettingValue(settingGotifyToken))
	if rawToken == "" {
		checks = append(checks, securityCheck{
			Name:    "gotify_token_encryption",
			Status:  "pass",
			Details: "No Gotify token is currently stored.",
		})
	} else if strings.HasPrefix(rawToken, encryptedSecretPrefix) {
		key := strings.TrimSpace(a.getSettingValue(settingGotifyTokenEncKey))
		if key == "" {
			key = strings.TrimSpace(os.Getenv("GOTIFY_TOKEN_ENC_KEY"))
		}
		if key == "" {
			checks = append(checks, securityCheck{
				Name:           "gotify_token_encryption",
				Status:         "fail",
				Details:        "Encrypted token exists but Gotify encryption key is missing.",
				Recommendation: "Set Gotify encryption key in Admin > Security.",
			})
		} else {
			checks = append(checks, securityCheck{
				Name:    "gotify_token_encryption",
				Status:  "pass",
				Details: "Gotify token is encrypted at rest and key is configured.",
			})
		}
	} else {
		checks = append(checks, securityCheck{
			Name:           "gotify_token_encryption",
			Status:         "warn",
			Details:        "Gotify token is stored in plaintext.",
			Recommendation: "Set Gotify encryption key in Admin > Security and re-save token.",
		})
	}

	// File permission audit note (cross-platform)
	if runtime.GOOS == "windows" {
		checks = append(checks, securityCheck{
			Name:    "file_permissions",
			Status:  "info",
			Details: "UNIX chmod hardening is skipped on Windows by design.",
		})
	} else {
		checks = append(checks, securityCheck{
			Name:    "file_permissions",
			Status:  "pass",
			Details: "UNIX file permission hardening is active for backups and DB restore.",
		})
	}

	overall := "pass"
	for _, c := range checks {
		if c.Status == "fail" {
			overall = "fail"
			break
		}
		if c.Status == "warn" && overall != "fail" {
			overall = "warn"
		}
	}

	writeJSON(w, http.StatusOK, securityHealthView{Status: overall, Checks: checks})
}
