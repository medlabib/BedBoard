package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func (a *App) initDatabase() error {
	db, err := gorm.Open(sqlite.Open(dbFileName), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&Bed{}, &Patient{}, &PatientEvent{}, &AdminUser{}, &Session{}, &AuditLog{}, &AppSetting{}, &AlertNotification{}); err != nil {
		return err
	}
	a.db = db
	return a.bootstrapData()
}

func (a *App) bootstrapData() error {
	var adminCount int64
	if err := a.db.Model(&AdminUser{}).Count(&adminCount).Error; err != nil {
		return err
	}
	if adminCount == 0 {
		username := strings.TrimSpace(a.getSettingValue(settingAdminInitUsername))
		if username == "" {
			username = strings.TrimSpace(os.Getenv("ADMIN_INIT_USERNAME"))
		}
		if username == "" {
			username = defaultUsername
		}
		password := strings.TrimSpace(a.getSettingValue(settingAdminInitPassword))
		if password == "" {
			password = strings.TrimSpace(os.Getenv("ADMIN_INIT_PASSWORD"))
		}
		if password == "" {
			password = insecureDefaultBootstrapPassword
			appLog.Warnw("security.admin_init_password not set; using insecure bootstrap password")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		now := time.Now()
		if err := a.db.Create(&AdminUser{Username: username, Role: roleAdmin, PasswordHash: string(hash), PasswordChangedAt: &now}).Error; err != nil {
			return err
		}
	}

	var users []AdminUser
	if err := a.db.Find(&users).Error; err != nil {
		return err
	}
	for _, user := range users {
		nextRole := roleOf(user)
		if user.Role != nextRole {
			user.Role = nextRole
			if err := a.db.Save(&user).Error; err != nil {
				return err
			}
		}
	}

	var bedCount int64
	if err := a.db.Model(&Bed{}).Count(&bedCount).Error; err != nil {
		return err
	}
	if bedCount == 0 {
		beds := defaultBeds()
		if err := a.db.Create(&beds).Error; err != nil {
			return err
		}
	}
	return nil
}

func defaultBeds() []Bed {
	return []Bed{
		{Number: 1, Room: "Chambre 1", Name: "Lit 1", Type: defaultBedType, Status: statusFree},
		{Number: 2, Room: "Chambre 1", Name: "Lit 2", Type: defaultBedType, Status: statusFree},
		{Number: 3, Room: "Chambre 2", Name: "Lit 3", Type: defaultBedType, Status: statusFree},
		{Number: 4, Room: "Chambre 2", Name: "Lit 4", Type: defaultBedType, Status: statusFree},
		{Number: 5, Room: "Chambre Thoracique", Name: "Lit 5", Type: thoracicBedType, Status: statusFree},
	}
}

func normalizePatientStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case patientStatusArrived:
		return patientStatusArrived
	case patientStatusTriaged:
		return patientStatusTriaged
	case patientStatusWaiting:
		return patientStatusWaiting
	case patientStatusAssigned:
		return patientStatusAssigned
	case patientStatusConsulted:
		return patientStatusConsulted
	case patientStatusArchived:
		return patientStatusArchived
	case patientStatusTransferred:
		return patientStatusTransferred
	case patientStatusDeceased:
		return patientStatusDeceased
	default:
		return ""
	}
}

func (a *App) logPatientEvent(registrationNumber, username, event string, details map[string]any) {
	if strings.TrimSpace(registrationNumber) == "" || strings.TrimSpace(event) == "" {
		return
	}
	serialized, _ := json.Marshal(details)
	entry := PatientEvent{
		RegistrationNumber: strings.TrimSpace(registrationNumber),
		Username:           fallback(strings.TrimSpace(username), "system"),
		Event:              strings.TrimSpace(event),
		Details:            string(serialized),
	}
	if err := a.db.Create(&entry).Error; err != nil {
		log.Printf("patient event log failed: %v", err)
	}
}

func (a *App) maybePublishTriageSLAAlert(patient Patient, actor string) {
	if patient.TriageScore < 4 || patient.ArrivedAt == nil || patient.AssignedAt != nil || patient.ConsultedAt != nil {
		return
	}
	triageSlaMinutes := a.getSettingInt(settingTriageSLAMinutes, envInt("TRIAGE_SLA_MINUTES", 15))
	if time.Since(*patient.ArrivedAt).Minutes() <= float64(triageSlaMinutes) {
		return
	}
	payload := alertPayload{
		Title:      "URGENT",
		Reason:     "triage_sla_breach",
		Patient:    fallback(patient.Name, patient.RegistrationNumber),
		Room:       "Non assignée",
		Bed:        "Non assigné",
		SourceUser: fallback(actor, "system"),
		TimeHM:     time.Now().Format("15:04"),
	}
	publishUrgentAlert(a, payload)
}

func envInt(name string, def int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

func envBool(name string, def bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
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

func passwordMinLength() int {
	return envInt("PASSWORD_MIN_LENGTH", 12)
}

func passwordMaxAgeDays() int {
	return envInt("PASSWORD_MAX_AGE_DAYS", 90)
}

func authMaxAttempts() int {
	return envInt("AUTH_MAX_ATTEMPTS", 5)
}

func authLockMinutes() int {
	return envInt("AUTH_LOCK_MINUTES", 15)
}

func validatePasswordPolicy(password string) error {
	if len(password) < passwordMinLength() || len(password) > 256 {
		return fmt.Errorf("password must be %d-256 characters", passwordMinLength())
	}
	needUpper := envBool("PASSWORD_REQUIRE_UPPER", true)
	needLower := envBool("PASSWORD_REQUIRE_LOWER", true)
	needDigit := envBool("PASSWORD_REQUIRE_DIGIT", true)
	needSymbol := envBool("PASSWORD_REQUIRE_SYMBOL", true)
	hasUpper, hasLower, hasDigit, hasSymbol := false, false, false, false
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			hasSymbol = true
		}
	}
	if needUpper && !hasUpper {
		return fmt.Errorf("password must include an uppercase letter")
	}
	if needLower && !hasLower {
		return fmt.Errorf("password must include a lowercase letter")
	}
	if needDigit && !hasDigit {
		return fmt.Errorf("password must include a digit")
	}
	if needSymbol && !hasSymbol {
		return fmt.Errorf("password must include a symbol")
	}
	return nil
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := a.currentUser(r)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": ok, "username": user.Username, "admin": isAdminLike(user), "role": roleOf(user)})
}

func (a *App) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) > 64 || len(req.Password) > 256 {
		http.Error(w, "invalid credentials", http.StatusBadRequest)
		return
	}
	if req.Username == "" {
		req.Username = defaultUsername
	}
	var user AdminUser
	if err := a.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	now := time.Now()
	if user.LockedUntil != nil && user.LockedUntil.After(now) {
		http.Error(w, "account locked due to failed attempts", http.StatusTooManyRequests)
		return
	}
	if !isAdminLike(user) && user.PasswordChangedAt != nil && passwordMaxAgeDays() > 0 {
		expiresAt := user.PasswordChangedAt.Add(time.Duration(passwordMaxAgeDays()) * 24 * time.Hour)
		if now.After(expiresAt) {
			http.Error(w, "password expired, ask admin to reset it", http.StatusForbidden)
			return
		}
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		user.FailedAttempts++
		if user.FailedAttempts >= authMaxAttempts() {
			lockedUntil := now.Add(time.Duration(authLockMinutes()) * time.Minute)
			user.LockedUntil = &lockedUntil
			user.FailedAttempts = 0
		}
		_ = a.db.Save(&user).Error
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	user.FailedAttempts = 0
	user.LockedUntil = nil
	_ = a.db.Save(&user).Error
	token, err := randomToken()
	if err != nil {
		http.Error(w, "token error", http.StatusInternalServerError)
		return
	}
	session := Session{Token: token, AdminUserID: user.ID, ExpiresAt: time.Now().Add(sessionDuration)}
	if err := a.db.Create(&session).Error; err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.shouldUseSecureCookie(r),
		SameSite: http.SameSiteStrictMode,
		Expires:  session.ExpiresAt,
		MaxAge:   int(sessionDuration.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "authenticated": true, "username": user.Username, "admin": isAdminLike(user), "role": roleOf(user)})
}

func (a *App) shouldUseSecureCookie(r *http.Request) bool {
	if r != nil && r.TLS != nil {
		return true
	}
	if a.getSettingBool(settingForceSecureCookie, envBool("FORCE_SECURE_COOKIE", false)) {
		return true
	}
	if !a.getSettingBool(settingTrustProxyHeaders, envBool("TRUST_PROXY_HEADERS", false)) || r == nil {
		return false
	}
	xfp := strings.TrimSpace(strings.ToLower(r.Header.Get("X-Forwarded-Proto")))
	if xfp == "" {
		return false
	}
	parts := strings.Split(xfp, ",")
	return strings.TrimSpace(parts[0]) == "https"
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		a.db.Where("token = ?", cookie.Value).Delete(&Session{})
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var users []AdminUser
		if err := a.db.Order("username asc").Find(&users).Error; err != nil {
			http.Error(w, "list failed", http.StatusInternalServerError)
			return
		}
		views := make([]userView, 0, len(users))
		for _, user := range users {
			role := roleOf(user)
			views = append(views, userView{Username: user.Username, Admin: isAdminLike(user), Role: role})
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": views})
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var req userRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		username := strings.TrimSpace(req.Username)
		password := strings.TrimSpace(req.Password)
		if username == "" || password == "" {
			http.Error(w, "username and password required", http.StatusBadRequest)
			return
		}
		if len(username) > 64 {
			http.Error(w, "invalid username", http.StatusBadRequest)
			return
		}
		if err := validatePasswordPolicy(password); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var existing AdminUser
		if err := a.db.Where("username = ?", username).First(&existing).Error; err == nil {
			http.Error(w, "user already exists", http.StatusConflict)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "password hash failed", http.StatusInternalServerError)
			return
		}
		now := time.Now()
		role := normalizeRole(req.Role)
		if username == defaultUsername {
			role = roleAdmin
		}
		user := AdminUser{Username: username, Role: role, PasswordHash: string(hash), PasswordChangedAt: &now}
		if err := a.db.Create(&user).Error; err != nil {
			http.Error(w, "create failed", http.StatusInternalServerError)
			return
		}
		a.broadcastEvent("user.updated", map[string]any{"username": user.Username})
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "user": userView{Username: user.Username, Admin: isAdminLike(user), Role: role}})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) handleUserPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	actor, ok := a.currentUser(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req passwordChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	targetUsername := strings.TrimSpace(req.Username)
	if targetUsername == "" {
		targetUsername = actor.Username
	}
	newPassword := strings.TrimSpace(req.NewPassword)
	if err := validatePasswordPolicy(newPassword); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	actorIsAdmin := roleOf(actor) == roleAdmin
	if targetUsername != actor.Username && !actorIsAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if targetUsername == actor.Username {
		if strings.TrimSpace(req.CurrentPassword) == "" {
			http.Error(w, "current password required", http.StatusBadRequest)
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(actor.PasswordHash), []byte(req.CurrentPassword)); err != nil {
			http.Error(w, "current password invalid", http.StatusUnauthorized)
			return
		}
	}
	var target AdminUser
	if err := a.db.Where("username = ?", targetUsername).First(&target).Error; err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "password hash failed", http.StatusInternalServerError)
		return
	}
	target.PasswordHash = string(hash)
	now := time.Now()
	target.PasswordChangedAt = &now
	target.FailedAttempts = 0
	target.LockedUntil = nil
	if err := a.db.Save(&target).Error; err != nil {
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}
	a.broadcastEvent("user.updated", map[string]any{"username": target.Username})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 200
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 && parsed <= 5000 {
			limit = parsed
		}
	}
	query := a.db.Model(&AuditLog{})
	if entity := strings.TrimSpace(r.URL.Query().Get("entity")); entity != "" {
		query = query.Where("entity = ?", entity)
	}
	if action := strings.TrimSpace(r.URL.Query().Get("action")); action != "" {
		query = query.Where("action = ?", action)
	}
	if username := strings.TrimSpace(r.URL.Query().Get("username")); username != "" {
		query = query.Where("username = ?", username)
	}
	if from := strings.TrimSpace(r.URL.Query().Get("from")); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if to := strings.TrimSpace(r.URL.Query().Get("to")); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			query = query.Where("created_at <= ?", t)
		}
	}
	var logs []AuditLog
	if err := query.Order("created_at desc").Limit(limit).Find(&logs).Error; err != nil {
		http.Error(w, "audit read failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
}

func (a *App) handleAuditExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := a.db.Model(&AuditLog{})
	if entity := strings.TrimSpace(r.URL.Query().Get("entity")); entity != "" {
		query = query.Where("entity = ?", entity)
	}
	if action := strings.TrimSpace(r.URL.Query().Get("action")); action != "" {
		query = query.Where("action = ?", action)
	}
	if username := strings.TrimSpace(r.URL.Query().Get("username")); username != "" {
		query = query.Where("username = ?", username)
	}
	var logs []AuditLog
	if err := query.Order("created_at desc").Limit(10000).Find(&logs).Error; err != nil {
		http.Error(w, "audit export failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=audit_export.csv")
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"createdAt", "username", "entity", "entityKey", "action", "oldValue", "newValue"})
	for _, logItem := range logs {
		_ = writer.Write([]string{
			logItem.CreatedAt.Format(time.RFC3339),
			logItem.Username,
			logItem.Entity,
			logItem.EntityKey,
			logItem.Action,
			logItem.OldValue,
			logItem.NewValue,
		})
	}
	writer.Flush()
}

func (a *App) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	state, err := a.collectState(r)
	if err != nil {
		http.Error(w, "state error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (a *App) handleStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	client := make(chan string, 8)
	a.registerClient(client)
	defer a.unregisterClient(client)

	payload, err := a.collectState(r)
	if err != nil {
		http.Error(w, "state error", http.StatusInternalServerError)
		return
	}
	data, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "event: state.snapshot\ndata: %s\n\n", data)
	flusher.Flush()

	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()

	ctx := r.Context()
	for {
		select {
		case msg := <-client:
			if _, err := fmt.Fprint(w, msg); err != nil {
				return
			}
			flusher.Flush()
		case <-keepAlive.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

func (a *App) handleBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	if err := os.MkdirAll(backupDirName, 0o700); err != nil {
		http.Error(w, "backup directory error", http.StatusInternalServerError)
		return
	}
	if err := ensurePrivateDirPermissions(backupDirName); err != nil {
		http.Error(w, "backup directory permission error", http.StatusInternalServerError)
		return
	}
	backupFile := filepath.Join(backupDirName, fmt.Sprintf("bedboard_%s.db", time.Now().Format("20060102_150405")))
	escaped := strings.ReplaceAll(backupFile, "'", "''")
	if err := a.db.Exec("VACUUM INTO '" + escaped + "'").Error; err != nil {
		http.Error(w, "backup failed", http.StatusInternalServerError)
		return
	}
	if err := ensurePrivateFilePermissions(backupFile); err != nil {
		http.Error(w, "backup file permission error", http.StatusInternalServerError)
		return
	}
	a.broadcastEvent("system.backup", map[string]any{"file": backupFile})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "file": backupFile})
}

func (a *App) handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req restoreRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	target := strings.TrimSpace(req.File)
	var err error
	if target == "" {
		latest, err := latestBackupFile()
		if err != nil {
			http.Error(w, "no backup found", http.StatusNotFound)
			return
		}
		target = latest
	}
	target, err = sanitizeBackupPath(target)
	if err != nil {
		http.Error(w, "invalid backup file", http.StatusBadRequest)
		return
	}
	if _, err := os.Stat(target); err != nil {
		http.Error(w, "backup not found", http.StatusNotFound)
		return
	}
	if a.db != nil {
		sqlDB, err := a.db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}
	if err := restoreDatabaseFile(target); err != nil {
		http.Error(w, "restore failed", http.StatusInternalServerError)
		return
	}
	if err := a.reopenDatabase(); err != nil {
		http.Error(w, "reopen failed", http.StatusInternalServerError)
		return
	}
	a.broadcastEvent("system.restore", map[string]any{"file": target})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "file": target})
}

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req statusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	number := req.Number
	if number == 0 {
		number = req.ID
	}
	if number == 0 {
		http.Error(w, "bed number required", http.StatusBadRequest)
		return
	}
	var bed Bed
	if err := a.db.Where("number = ?", number).First(&bed).Error; err != nil {
		http.Error(w, "bed not found", http.StatusNotFound)
		return
	}
	user, _ := a.currentUser(r)
	if !canManageBeds(user) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	before := bed
	alertSnapshot := bed
	if alertSnapshot.PatientID != nil {
		var currentPatient Patient
		if err := a.db.First(&currentPatient, *alertSnapshot.PatientID).Error; err == nil && strings.TrimSpace(currentPatient.Name) != "" {
			alertSnapshot.PatientName = currentPatient.Name
		}
	}
	bed.Status = normalizeStatus(req.Status)
	if req.PatientName != "" {
		bed.PatientName = req.PatientName
		if strings.TrimSpace(req.PatientName) != "" {
			alertSnapshot.PatientName = strings.TrimSpace(req.PatientName)
		}
	}
	if bed.Status == statusFree || bed.Status == statusCleaning {
		if bed.PatientID != nil {
			markConsulted := bed.Status == statusFree || bed.Status == statusCleaning
			a.releasePatientByID(*bed.PatientID, user.Username, markConsulted)
		}
		bed.PatientID = nil
		bed.PatientName = ""
	}
	if bed.Status == statusOccupied && bed.Time == "" {
		bed.Time = time.Now().Format("15:04")
	}
	if err := a.db.Save(&bed).Error; err != nil {
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}
	a.logBedChange(user.Username, "bed.status", before, bed)
	if bed.Status == statusAlert {
		publishUrgentAlert(a, buildBedAlertPayload(alertSnapshot, user.Username))
	}
	a.broadcastEvent("bed.updated", map[string]any{"number": bed.Number})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleConfigBed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req bedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Number == 0 {
		http.Error(w, "bed number required", http.StatusBadRequest)
		return
	}
	var bed Bed
	if err := a.db.Where("number = ?", req.Number).First(&bed).Error; err != nil {
		http.Error(w, "bed not found", http.StatusNotFound)
		return
	}
	user, _ := a.currentUser(r)
	if !canManageBeds(user) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	before := bed
	if strings.TrimSpace(req.Name) != "" {
		bed.Name = strings.TrimSpace(req.Name)
	}
	if strings.TrimSpace(req.NameAlt) != "" {
		bed.NameAlt = strings.TrimSpace(req.NameAlt)
	}
	if strings.TrimSpace(req.Room) != "" {
		bed.Room = strings.TrimSpace(req.Room)
	}
	if strings.TrimSpace(req.RoomAlt) != "" {
		bed.RoomAlt = strings.TrimSpace(req.RoomAlt)
	}
	if strings.TrimSpace(req.Type) != "" {
		bed.Type = normalizeType(req.Type)
	}
	if err := a.db.Save(&bed).Error; err != nil {
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}
	a.logBedChange(user.Username, "bed.config", before, bed)
	a.broadcastEvent("bed.updated", map[string]any{"number": bed.Number})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleBedsCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req bedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Number == 0 {
		http.Error(w, "bed number required", http.StatusBadRequest)
		return
	}
	user, _ := a.currentUser(r)
	if !canManageBeds(user) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var existing Bed
	if err := a.db.Where("number = ?", req.Number).First(&existing).Error; err == nil {
		http.Error(w, "bed already exists", http.StatusConflict)
		return
	}
	bed := Bed{
		Number:  req.Number,
		Room:    fallback(strings.TrimSpace(req.Room), fmt.Sprintf("Chambre %d", req.Number)),
		RoomAlt: strings.TrimSpace(req.RoomAlt),
		Name:    fallback(strings.TrimSpace(req.Name), fmt.Sprintf("Lit %d", req.Number)),
		NameAlt: strings.TrimSpace(req.NameAlt),
		Type:    normalizeType(req.Type),
		Status:  statusFree,
	}
	if err := a.db.Create(&bed).Error; err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	a.logBedChange(user.Username, "bed.create", Bed{}, bed)
	a.broadcastEvent("bed.created", map[string]any{"number": bed.Number})
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (a *App) handleBedsDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req bedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Number == 0 {
		http.Error(w, "bed number required", http.StatusBadRequest)
		return
	}
	var bed Bed
	if err := a.db.Where("number = ?", req.Number).First(&bed).Error; err != nil {
		http.Error(w, "bed not found", http.StatusNotFound)
		return
	}
	user, _ := a.currentUser(r)
	before := bed
	if bed.PatientID != nil {
		a.releasePatientByID(*bed.PatientID, user.Username, false)
	}
	if err := a.db.Delete(&bed).Error; err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	a.logBedChange(user.Username, "bed.delete", before, Bed{})
	a.broadcastEvent("bed.deleted", map[string]any{"number": before.Number})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handlePatients(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var req patientRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.RegistrationNumber) == "" {
			http.Error(w, "registration number required", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.PatientType) != "" && normalizePatientType(req.PatientType) == "" {
			http.Error(w, "patient type must be one of: traumato, medical, douleurs thoracique, chirurgical, urgences differees", http.StatusBadRequest)
			return
		}
		user, _ := a.currentUser(r)
		if !canManagePatients(user) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		previousTriageScore := -1
		var existingPatient Patient
		if err := a.db.Where("registration_number = ?", strings.TrimSpace(req.RegistrationNumber)).First(&existingPatient).Error; err == nil {
			previousTriageScore = existingPatient.TriageScore
		} else if err != gorm.ErrRecordNotFound {
			http.Error(w, "patient lookup failed", http.StatusInternalServerError)
			return
		}
		if req.TriageScore != nil && (*req.TriageScore < 0 || *req.TriageScore > 4) {
			http.Error(w, "triage score must be 0..4", http.StatusBadRequest)
			return
		}
		if roleOf(user) == roleTriage && (req.BedNumber > 0 || req.BedID > 0) {
			http.Error(w, "triage cannot assign beds", http.StatusForbidden)
			return
		}
		patient, err := a.upsertAndAssignPatient(req, user.Username)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Status) != "" {
			nextStatus := normalizePatientStatus(req.Status)
			if nextStatus == "" {
				http.Error(w, "invalid patient status", http.StatusBadRequest)
				return
			}
			patient.Status = nextStatus
		}
		if strings.TrimSpace(req.Reason) != "" {
			patient.Reason = strings.TrimSpace(req.Reason)
		}
		if strings.TrimSpace(req.Destination) != "" {
			patient.Destination = strings.TrimSpace(req.Destination)
		}
		if strings.TrimSpace(req.Outcome) != "" {
			patient.Outcome = strings.TrimSpace(req.Outcome)
		}
		now := time.Now()
		if patient.ArrivedAt == nil {
			patient.ArrivedAt = &now
		}
		if req.TriageScore != nil && patient.TriagedAt == nil {
			patient.TriagedAt = &now
		}
		if patient.Status == patientStatusAssigned && patient.StartedAt == nil {
			patient.StartedAt = &now
		}
		if patient.Status == patientStatusTransferred || patient.Status == patientStatusDeceased {
			patient.ExitAt = &now
		}
		if err := a.db.Save(&patient).Error; err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		if req.TriageScore != nil && patient.TriageScore == 4 && previousTriageScore != 4 {
			var linkedBed *Bed
			if patient.BedID != nil {
				var bed Bed
				if err := a.db.First(&bed, *patient.BedID).Error; err == nil {
					linkedBed = &bed
				}
			}
			publishUrgentAlert(a, buildTriageAlertPayload(patient, linkedBed, user.Username))
		}
		a.maybePublishTriageSLAAlert(patient, user.Username)
		a.logPatientEvent(patient.RegistrationNumber, user.Username, "patient.saved", map[string]any{
			"status":      patient.Status,
			"triageScore": patient.TriageScore,
			"bedAssigned": patient.BedID != nil,
		})
		a.broadcastEvent("patient.updated", map[string]any{"registrationNumber": patient.RegistrationNumber})
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "patient": patient})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) handlePatientEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	registrationNumber := strings.TrimSpace(r.URL.Query().Get("registrationNumber"))
	if registrationNumber == "" {
		http.Error(w, "registration number required", http.StatusBadRequest)
		return
	}
	limit := 200
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}
	var events []PatientEvent
	if err := a.db.Where("lower(registration_number) = lower(?)", registrationNumber).Order("created_at desc").Limit(limit).Find(&events).Error; err != nil {
		http.Error(w, "events read failed", http.StatusInternalServerError)
		return
	}
	if len(events) == 0 {
		var patient Patient
		if err := a.db.Where("lower(registration_number) = lower(?)", registrationNumber).First(&patient).Error; err == nil {
			fallback := make([]PatientEvent, 0, 8)
			appendLifecycle := func(ts *time.Time, event string, details map[string]any) {
				if ts == nil || ts.IsZero() {
					return
				}
				raw, _ := json.Marshal(details)
				fallback = append(fallback, PatientEvent{
					RegistrationNumber: patient.RegistrationNumber,
					Username:           "system",
					Event:              event,
					Details:            string(raw),
					CreatedAt:          *ts,
				})
			}
			appendLifecycle(&patient.CreatedAt, "patient.created", map[string]any{"source": "legacy"})
			appendLifecycle(patient.ArrivedAt, "patient.arrived", map[string]any{"status": patient.Status})
			appendLifecycle(patient.TriagedAt, "patient.triaged", map[string]any{"triageScore": patient.TriageScore})
			appendLifecycle(patient.AssignedAt, "patient.assigned", map[string]any{"source": "legacy"})
			appendLifecycle(patient.StartedAt, "patient.started", map[string]any{"source": "legacy"})
			appendLifecycle(patient.ConsultedAt, "patient.consulted", map[string]any{"source": "legacy"})
			appendLifecycle(patient.ExitAt, "patient.exited", map[string]any{"status": patient.Status})
			appendLifecycle(patient.ArchivedAt, "patient.archived", map[string]any{"status": patient.Status})
			sort.Slice(fallback, func(i, j int) bool {
				return fallback[i].CreatedAt.After(fallback[j].CreatedAt)
			})
			if len(fallback) > limit {
				fallback = fallback[:limit]
			}
			events = fallback
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (a *App) handlePatientsImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	var req struct {
		Source   string           `json:"source"`
		Patients []patientRequest `json:"patients"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if len(req.Patients) == 0 {
		http.Error(w, "patients required", http.StatusBadRequest)
		return
	}
	if len(req.Patients) > 1000 {
		http.Error(w, "too many patients in one import", http.StatusBadRequest)
		return
	}
	user, _ := a.currentUser(r)
	processed := 0
	failures := make([]map[string]any, 0)
	for _, item := range req.Patients {
		item.RegistrationNumber = strings.TrimSpace(item.RegistrationNumber)
		if item.RegistrationNumber == "" {
			failures = append(failures, map[string]any{"registrationNumber": "", "error": "missing registration number"})
			continue
		}
		if _, err := a.upsertAndAssignPatient(item, user.Username); err != nil {
			failures = append(failures, map[string]any{"registrationNumber": item.RegistrationNumber, "error": err.Error()})
			continue
		}
		processed++
		a.logPatientEvent(item.RegistrationNumber, user.Username, "patient.imported", map[string]any{"source": strings.TrimSpace(req.Source)})
	}
	a.broadcastEvent("patient.updated", map[string]any{"source": "import", "processed": processed})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "processed": processed, "failed": len(failures), "failures": failures})
}

func (a *App) handlePatientsArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		RegistrationNumber string `json:"registrationNumber"`
		Action             string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.RegistrationNumber) == "" {
		http.Error(w, "registration number required", http.StatusBadRequest)
		return
	}
	var patient Patient
	if err := a.db.Where("registration_number = ?", strings.TrimSpace(req.RegistrationNumber)).First(&patient).Error; err != nil {
		http.Error(w, "patient not found", http.StatusNotFound)
		return
	}
	user, _ := a.currentUser(r)
	if !canArchivePatients(user) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	now := time.Now()
	switch req.Action {
	case "archive", "sortant":
		if patient.BedID != nil {
			a.releasePatientByID(patient.ID, user.Username, false)
			if err := a.db.First(&patient, patient.ID).Error; err != nil {
				http.Error(w, "patient reload failed", http.StatusInternalServerError)
				return
			}
		}
		patient.ArchivedAt = &now
		patient.Status = patientStatusArchived
		patient.ExitAt = &now
	case "consulted":
		patient.ConsultedAt = &now
		patient.Status = patientStatusConsulted
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	if err := a.db.Save(&patient).Error; err != nil {
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}
	a.logPatientEvent(patient.RegistrationNumber, user.Username, "patient.archive", map[string]any{"action": req.Action, "status": patient.Status})
	a.broadcastEvent("patient.archived", map[string]any{"registrationNumber": patient.RegistrationNumber})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "patient": patient})
}

func (a *App) upsertAndAssignPatient(req patientRequest, actor string) (Patient, error) {
	lockKey := 0
	if req.BedNumber > 0 {
		lockKey = req.BedNumber
	} else if req.BedID > 0 {
		lockKey = -req.BedID
	}
	if lockKey != 0 {
		unlock := a.lockBed(lockKey)
		defer unlock()
	}
	trimmedReg := strings.TrimSpace(req.RegistrationNumber)
	trimmedName := strings.TrimSpace(req.Name)
	trimmedReason := strings.TrimSpace(req.Reason)
	trimmedDestination := strings.TrimSpace(req.Destination)
	trimmedOutcome := strings.TrimSpace(req.Outcome)
	normalizedPatientType := ""
	if strings.TrimSpace(req.PatientType) != "" {
		normalizedPatientType = normalizePatientType(req.PatientType)
		if normalizedPatientType == "" {
			return Patient{}, fmt.Errorf("invalid patient type")
		}
	}
	isNew := false
	var patient Patient
	result := a.db.Where("registration_number = ?", trimmedReg).Limit(1).Find(&patient)
	if result.Error != nil {
		return Patient{}, result.Error
	}
	if result.RowsAffected == 0 {
		isNew = true
		patient = Patient{RegistrationNumber: trimmedReg}
	}
	if isNew && trimmedName == "" {
		return Patient{}, fmt.Errorf("name required for new patient")
	}
	if trimmedName != "" {
		patient.Name = trimmedName
	}
	if normalizedPatientType != "" {
		patient.PatientType = normalizedPatientType
	}
	if patient.PatientType == "" {
		if isNew {
			return Patient{}, fmt.Errorf("patient type required")
		}
		patient.PatientType = patientTypeMedical
	}
	if req.TriageScore != nil {
		patient.TriageScore = *req.TriageScore
	}
	if strings.TrimSpace(patient.Name) == "" {
		return Patient{}, fmt.Errorf("name required")
	}
	if trimmedReason != "" {
		patient.Reason = trimmedReason
	}
	if trimmedDestination != "" {
		patient.Destination = trimmedDestination
	}
	if trimmedOutcome != "" {
		patient.Outcome = trimmedOutcome
	}
	now := time.Now()
	if patient.ArrivedAt == nil {
		patient.ArrivedAt = &now
	}
	if req.TriageScore != nil && patient.TriagedAt == nil {
		patient.TriagedAt = &now
	}
	if patient.Status == "" {
		patient.Status = patientStatusArrived
	}
	if err := a.db.Save(&patient).Error; err != nil {
		return Patient{}, err
	}
	if req.BedNumber == 0 && req.BedID == 0 {
		return patient, nil
	}
	var bed Bed
	if req.BedNumber > 0 {
		if err := a.db.Where("number = ?", req.BedNumber).First(&bed).Error; err != nil {
			return Patient{}, fmt.Errorf("bed not found")
		}
	} else {
		if err := a.db.First(&bed, req.BedID).Error; err != nil {
			return Patient{}, fmt.Errorf("bed not found")
		}
	}
	if err := a.db.Where("id = ?", bed.ID).First(&bed).Error; err != nil {
		return Patient{}, fmt.Errorf("bed not found")
	}
	before := bed
	if bed.PatientID != nil && *bed.PatientID != patient.ID {
		return Patient{}, fmt.Errorf("bed already assigned")
	}
	if patient.BedID != nil && *patient.BedID != bed.ID {
		var previousBed Bed
		if err := a.db.First(&previousBed, *patient.BedID).Error; err == nil {
			previousBed.PatientID = nil
			previousBed.PatientName = ""
			previousBed.Status = statusFree
			_ = a.db.Save(&previousBed).Error
		}
	}
	patient.BedID = &bed.ID
	patient.AssignedAt = &now
	patient.Status = patientStatusAssigned
	if patient.StartedAt == nil {
		patient.StartedAt = &now
	}
	if err := a.db.Save(&patient).Error; err != nil {
		return Patient{}, err
	}
	bed.PatientID = &patient.ID
	bed.PatientName = patient.Name
	bed.Status = statusOccupied
	bed.Time = time.Now().Format("15:04")
	if err := a.db.Save(&bed).Error; err != nil {
		return Patient{}, err
	}
	a.logBedChange(actor, "bed.assign", before, bed)
	a.logPatientEvent(patient.RegistrationNumber, actor, "patient.assigned", map[string]any{"bedNumber": bed.Number, "room": bed.Room})
	return patient, nil
}

func (a *App) releasePatientByID(patientID uint, actor string, markConsulted bool) {
	var patient Patient
	if err := a.db.First(&patient, patientID).Error; err != nil {
		return
	}
	if patient.BedID != nil {
		var bed Bed
		if err := a.db.First(&bed, *patient.BedID).Error; err == nil {
			before := bed
			bed.PatientID = nil
			bed.PatientName = ""
			bed.Status = statusFree
			_ = a.db.Save(&bed).Error
			a.logBedChange(actor, "bed.release", before, bed)
		}
	}
	if markConsulted && patient.BedID != nil {
		now := time.Now()
		patient.ConsultedAt = &now
		patient.Status = patientStatusConsulted
	}
	patient.BedID = nil
	_ = a.db.Save(&patient).Error
	a.logPatientEvent(patient.RegistrationNumber, actor, "patient.released", map[string]any{"consulted": markConsulted})
}
