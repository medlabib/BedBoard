package main

import (
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

//go:embed frontend/dist logo.png
var embeddedFiles embed.FS

const (
	defaultPort       = ":8080"
	dbFileName        = "bedboard.db"
	legacyConfigFile  = "config_salle.json"
	backupDirName     = "backups"
	sessionCookieName = "bedboard_session"
	sessionDuration   = 7 * 24 * time.Hour
	defaultUsername   = "admin"
	defaultPassword   = "admin123"
	defaultBedType    = "standard"
	thoracicBedType   = "thoracique"
	statusFree        = "libre"
	statusOccupied    = "occupé"
	statusCleaning    = "nettoyage"
	statusAlert       = "alerte"
)

type Bed struct {
	ID          uint      `gorm:"primaryKey" json:"-"`
	Number      int       `gorm:"uniqueIndex;not null" json:"number"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	Time        string    `json:"time"`
	PatientID   *uint     `json:"-"`
	PatientName string    `json:"patientName"`
	CreatedAt   time.Time `json:"-"`
	UpdatedAt   time.Time `json:"-"`
}

type Patient struct {
	ID                 uint       `gorm:"primaryKey" json:"-"`
	RegistrationNumber string     `gorm:"uniqueIndex;not null" json:"registrationNumber"`
	Name               string     `json:"name"`
	BedID              *uint      `json:"-"`
	Status             string     `json:"status"`
	AssignedAt         *time.Time `json:"assignedAt"`
	ConsultedAt        *time.Time `json:"consultedAt"`
	ArchivedAt         *time.Time `json:"archivedAt"`
	ExitAt             *time.Time `json:"exitAt"`
	CreatedAt          time.Time  `json:"-"`
	UpdatedAt          time.Time  `json:"-"`
}

type AdminUser struct {
	ID                uint       `gorm:"primaryKey" json:"-"`
	Username          string     `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash      string     `json:"-"`
	PasswordChangedAt *time.Time `json:"-"`
	FailedAttempts    int        `json:"-"`
	LockedUntil       *time.Time `json:"-"`
	CreatedAt         time.Time  `json:"-"`
	UpdatedAt         time.Time  `json:"-"`
}

type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	Username  string    `json:"username"`
	Entity    string    `json:"entity"`
	EntityKey string    `json:"entityKey"`
	Action    string    `json:"action"`
	OldValue  string    `json:"oldValue"`
	NewValue  string    `json:"newValue"`
	CreatedAt time.Time `json:"createdAt"`
}

type Session struct {
	ID          uint      `gorm:"primaryKey" json:"-"`
	Token       string    `gorm:"uniqueIndex;not null" json:"-"`
	AdminUserID uint      `json:"-"`
	ExpiresAt   time.Time `json:"-"`
	CreatedAt   time.Time `json:"-"`
	UpdatedAt   time.Time `json:"-"`
}

type legacyConfig struct {
	AdminPassword string `json:"admin_password"`
	Beds          []struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Type    string `json:"type"`
		Status  string `json:"status"`
		Patient string `json:"patient"`
		Time    string `json:"time"`
	} `json:"beds"`
}

type App struct {
	db            *gorm.DB
	clientsMu     sync.Mutex
	clients       map[chan string]struct{}
	bedLocksMu    sync.Mutex
	bedLocks      map[int]*sync.Mutex
	maintenanceMu sync.Mutex
}

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type passwordChangeRequest struct {
	Username        string `json:"username"`
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type restoreRequest struct {
	File string `json:"file"`
}

type userView struct {
	Username string `json:"username"`
	Admin    bool   `json:"admin"`
}

type statusRequest struct {
	Number      int    `json:"number"`
	ID          int    `json:"id"`
	Status      string `json:"status"`
	PatientName string `json:"patientName"`
}

type bedRequest struct {
	Number int    `json:"number"`
	Name   string `json:"name"`
	Type   string `json:"type"`
}

type patientRequest struct {
	RegistrationNumber string `json:"registrationNumber"`
	Name               string `json:"name"`
	BedNumber          int    `json:"bedNumber"`
	BedID              int    `json:"bedId"`
}

type statePayload struct {
	Beds          []bedView     `json:"beds"`
	Patients      []patientView `json:"patients"`
	Stats         statsView     `json:"stats"`
	Authenticated bool          `json:"authenticated"`
	Username      string        `json:"username"`
	Admin         bool          `json:"admin"`
	ServerTime    string        `json:"serverTime"`
}

type bedView struct {
	Number              int    `json:"number"`
	Name                string `json:"name"`
	Type                string `json:"type"`
	Status              string `json:"status"`
	Time                string `json:"time"`
	PatientName         string `json:"patientName"`
	PatientRegistration string `json:"patientRegistration"`
	HasPatient          bool   `json:"hasPatient"`
}

type patientView struct {
	RegistrationNumber string     `json:"registrationNumber"`
	Name               string     `json:"name"`
	BedNumber          *int       `json:"bedNumber"`
	BedName            string     `json:"bedName"`
	Status             string     `json:"status"`
	AssignedAt         *time.Time `json:"assignedAt"`
}

type statsView struct {
	TotalBeds              int              `json:"totalBeds"`
	FreeBeds               int              `json:"freeBeds"`
	OccupiedBeds           int              `json:"occupiedBeds"`
	CleaningBeds           int              `json:"cleaningBeds"`
	AlertBeds              int              `json:"alertBeds"`
	TotalPatients          int              `json:"totalPatients"`
	ArchivedPatients       int              `json:"archivedPatients"`
	ConsultationsByDate    []map[string]any `json:"consultationsByDate"`
	AvgConsultationMinutes float64          `json:"avgConsultationMinutes"`
	TotalConsultations     int              `json:"totalConsultations"`
}

func main() {
	app := &App{clients: make(map[chan string]struct{}), bedLocks: make(map[int]*sync.Mutex)}
	if err := app.initDatabase(); err != nil {
		log.Fatalf("init database: %v", err)
	}

	distFS, err := fs.Sub(embeddedFiles, "frontend/dist")
	if err != nil {
		log.Fatalf("read embedded frontend: %v", err)
	}
	spaHandler := http.FileServer(http.FS(distFS))

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			spaHandler.ServeHTTP(w, r)
			return
		}
		if _, err := fs.Stat(distFS, strings.TrimPrefix(r.URL.Path, "/")); err == nil {
			spaHandler.ServeHTTP(w, r)
			return
		}
		http.ServeFileFS(w, r, distFS, "index.html")
	})
	mux.HandleFunc("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		logoFile, err := fs.ReadFile(embeddedFiles, "logo.png")
		if err != nil {
			http.Error(w, "logo not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(logoFile)
	})
	mux.HandleFunc("/api/me", app.withCORS(app.handleMe))
	mux.HandleFunc("/api/auth", app.withCORS(app.handleAuth))
	mux.HandleFunc("/api/logout", app.withCORS(app.handleLogout))
	mux.HandleFunc("/api/state", app.withCORS(app.handleState))
	mux.HandleFunc("/api/stream", app.withCORS(app.handleStream))
	mux.HandleFunc("/api/users", app.withCORS(app.requireAuth(app.requireAdmin(app.handleUsers))))
	mux.HandleFunc("/api/users/password", app.withCORS(app.requireAuth(app.handleUserPassword)))
	mux.HandleFunc("/api/audit", app.withCORS(app.requireAuth(app.requireAdmin(app.handleAudit))))
	mux.HandleFunc("/api/admin/backup", app.withCORS(app.requireAuth(app.requireAdmin(app.handleBackup))))
	mux.HandleFunc("/api/admin/restore", app.withCORS(app.requireAuth(app.requireAdmin(app.handleRestore))))
	mux.HandleFunc("/api/status", app.withCORS(app.requireAuth(app.handleStatus)))
	mux.HandleFunc("/api/config-bed", app.withCORS(app.requireAuth(app.handleConfigBed)))
	mux.HandleFunc("/api/beds", app.withCORS(app.requireAuth(app.handleBedsCreate)))
	mux.HandleFunc("/api/beds/delete", app.withCORS(app.requireAuth(app.requireAdmin(app.handleBedsDelete))))
	mux.HandleFunc("/api/patients", app.withCORS(app.requireAuth(app.handlePatients)))
	mux.HandleFunc("/api/patients/archive", app.withCORS(app.requireAuth(app.handlePatientsArchive)))

	serverAddr := defaultPort
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		if strings.HasPrefix(port, ":") {
			serverAddr = port
		} else {
			serverAddr = ":" + port
		}
	}

	handler := app.withSecurityHeaders(mux)

	log.Printf("BedBoard listening on http://localhost%s", serverAddr)
	if err := http.ListenAndServe(serverAddr, handler); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
}

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
	if err := db.AutoMigrate(&Bed{}, &Patient{}, &AdminUser{}, &Session{}, &AuditLog{}); err != nil {
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
		username := defaultUsername
		password := defaultPassword
		if legacy, err := readLegacyConfig(); err == nil && legacy.AdminPassword != "" {
			password = legacy.AdminPassword
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		now := time.Now()
		if err := a.db.Create(&AdminUser{Username: username, PasswordHash: string(hash), PasswordChangedAt: &now}).Error; err != nil {
			return err
		}
	}

	var bedCount int64
	if err := a.db.Model(&Bed{}).Count(&bedCount).Error; err != nil {
		return err
	}
	if bedCount == 0 {
		if legacy, err := readLegacyConfig(); err == nil && len(legacy.Beds) > 0 {
			beds := make([]Bed, 0, len(legacy.Beds))
			for _, item := range legacy.Beds {
				bed := Bed{
					Number:      item.ID,
					Name:        fallback(item.Name, fmt.Sprintf("Lit %d", item.ID)),
					Type:        normalizeType(item.Type),
					Status:      normalizeStatus(item.Status),
					Time:        item.Time,
					PatientName: item.Patient,
				}
				beds = append(beds, bed)
			}
			if err := a.db.Create(&beds).Error; err != nil {
				return err
			}
		} else {
			beds := defaultBeds()
			if err := a.db.Create(&beds).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func readLegacyConfig() (*legacyConfig, error) {
	data, err := os.ReadFile(legacyConfigFile)
	if err != nil {
		return nil, err
	}
	var cfg legacyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func defaultBeds() []Bed {
	return []Bed{
		{Number: 1, Name: "Lit 1", Type: defaultBedType, Status: statusFree},
		{Number: 2, Name: "Lit 2", Type: defaultBedType, Status: statusFree},
		{Number: 3, Name: "Lit 3", Type: defaultBedType, Status: statusFree},
		{Number: 4, Name: "Lit 4", Type: defaultBedType, Status: statusFree},
		{Number: 5, Name: "Lit 5", Type: thoracicBedType, Status: statusFree},
	}
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
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": ok, "username": user.Username, "admin": user.Username == defaultUsername})
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
	if user.Username != defaultUsername && user.PasswordChangedAt != nil && passwordMaxAgeDays() > 0 {
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
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		Expires:  session.ExpiresAt,
		MaxAge:   int(sessionDuration.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "authenticated": true, "username": user.Username, "admin": user.Username == defaultUsername})
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
			views = append(views, userView{Username: user.Username, Admin: user.Username == defaultUsername})
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
		user := AdminUser{Username: username, PasswordHash: string(hash), PasswordChangedAt: &now}
		if err := a.db.Create(&user).Error; err != nil {
			http.Error(w, "create failed", http.StatusInternalServerError)
			return
		}
		a.broadcastEvent("user.updated", map[string]any{"username": user.Username})
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "user": userView{Username: user.Username, Admin: false}})
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
	actorIsAdmin := actor.Username == defaultUsername
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
	var logs []AuditLog
	if err := a.db.Order("created_at desc").Limit(200).Find(&logs).Error; err != nil {
		http.Error(w, "audit read failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
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
	if err := os.MkdirAll(backupDirName, 0o755); err != nil {
		http.Error(w, "backup directory error", http.StatusInternalServerError)
		return
	}
	backupFile := filepath.Join(backupDirName, fmt.Sprintf("bedboard_%s.db", time.Now().Format("20060102_150405")))
	escaped := strings.ReplaceAll(backupFile, "'", "''")
	if err := a.db.Exec("VACUUM INTO '" + escaped + "'").Error; err != nil {
		http.Error(w, "backup failed", http.StatusInternalServerError)
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
	if target == "" {
		latest, err := latestBackupFile()
		if err != nil {
			http.Error(w, "no backup found", http.StatusNotFound)
			return
		}
		target = latest
	}
	if !strings.HasPrefix(target, backupDirName+string(os.PathSeparator)) && !strings.HasPrefix(target, backupDirName+"/") {
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
	before := bed
	bed.Status = normalizeStatus(req.Status)
	if req.PatientName != "" {
		bed.PatientName = req.PatientName
	}
	if bed.Status == statusFree || bed.Status == statusCleaning || bed.Status == statusAlert {
		if bed.PatientID != nil {
			a.releasePatientByID(*bed.PatientID, user.Username)
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
	before := bed
	if strings.TrimSpace(req.Name) != "" {
		bed.Name = strings.TrimSpace(req.Name)
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
	var existing Bed
	if err := a.db.Where("number = ?", req.Number).First(&existing).Error; err == nil {
		http.Error(w, "bed already exists", http.StatusConflict)
		return
	}
	bed := Bed{
		Number: req.Number,
		Name:   fallback(strings.TrimSpace(req.Name), fmt.Sprintf("Lit %d", req.Number)),
		Type:   normalizeType(req.Type),
		Status: statusFree,
	}
	if err := a.db.Create(&bed).Error; err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	user, _ := a.currentUser(r)
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
		a.releasePatientByID(*bed.PatientID, user.Username)
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
		user, _ := a.currentUser(r)
		patient, err := a.upsertAndAssignPatient(req, user.Username)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.broadcastEvent("patient.updated", map[string]any{"registrationNumber": patient.RegistrationNumber})
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "patient": patient})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
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
	now := time.Now()
	switch req.Action {
	case "archive", "sortant":
		patient.ArchivedAt = &now
		patient.Status = "archived"
		patient.ExitAt = &now
	case "consulted":
		patient.ConsultedAt = &now
		patient.Status = "consulted"
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	if err := a.db.Save(&patient).Error; err != nil {
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}
	a.broadcastEvent("patient.archived", map[string]any{"registrationNumber": patient.RegistrationNumber})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "patient": patient})
}

func (a *App) upsertAndAssignPatient(req patientRequest, actor string) (Patient, error) {
	bedNumber := req.BedNumber
	if bedNumber == 0 {
		bedNumber = req.BedID
	}
	if bedNumber > 0 {
		unlock := a.lockBed(bedNumber)
		defer unlock()
	}
	trimmedReg := strings.TrimSpace(req.RegistrationNumber)
	trimmedName := strings.TrimSpace(req.Name)
	isNew := false
	var patient Patient
	if err := a.db.Where("registration_number = ?", trimmedReg).First(&patient).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return Patient{}, err
		}
		isNew = true
		patient = Patient{RegistrationNumber: trimmedReg}
	}
	if isNew && trimmedName == "" {
		return Patient{}, fmt.Errorf("name required for new patient")
	}
	if trimmedName != "" {
		patient.Name = trimmedName
	}
	if strings.TrimSpace(patient.Name) == "" {
		return Patient{}, fmt.Errorf("name required")
	}
	if patient.Status == "" {
		patient.Status = "unassigned"
	}
	if err := a.db.Save(&patient).Error; err != nil {
		return Patient{}, err
	}
	if bedNumber == 0 {
		return patient, nil
	}
	var bed Bed
	if err := a.db.Where("number = ?", bedNumber).First(&bed).Error; err != nil {
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
	now := time.Now()
	patient.BedID = &bed.ID
	patient.AssignedAt = &now
	patient.Status = "assigned"
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
	return patient, nil
}

func (a *App) releasePatientByID(patientID uint, actor string) {
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
	// mark consultation done when the patient is released from bed
	now := time.Now()
	if patient.BedID != nil {
		patient.ConsultedAt = &now
		patient.Status = "consulted"
	}
	patient.BedID = nil
	_ = a.db.Save(&patient).Error
}

func (a *App) collectState(r *http.Request) (statePayload, error) {
	beds, err := a.listBeds()
	if err != nil {
		return statePayload{}, err
	}
	patients, err := a.listPatients()
	if err != nil {
		return statePayload{}, err
	}
	stats := statsView{TotalBeds: len(beds), TotalPatients: len(patients)}
	bedByID := make(map[uint]Bed, len(beds))
	patientByID := make(map[uint]Patient, len(patients))
	for _, bed := range beds {
		bedByID[bed.ID] = bed
		switch normalizeStatus(bed.Status) {
		case statusOccupied:
			stats.OccupiedBeds++
		case statusCleaning:
			stats.CleaningBeds++
		case statusAlert:
			stats.AlertBeds++
		default:
			stats.FreeBeds++
		}
	}
	for _, patient := range patients {
		patientByID[patient.ID] = patient
	}
	views := make([]bedView, 0, len(beds))
	for _, bed := range beds {
		view := bedView{
			Number:      bed.Number,
			Name:        bed.Name,
			Type:        normalizeType(bed.Type),
			Status:      normalizeStatus(bed.Status),
			Time:        bed.Time,
			PatientName: bed.PatientName,
		}
		if bed.PatientID != nil {
			if patient, ok := patientByID[*bed.PatientID]; ok {
				view.PatientName = patient.Name
				view.PatientRegistration = patient.RegistrationNumber
				view.HasPatient = true
			}
		}
		views = append(views, view)
	}
	patientViews := make([]patientView, 0, len(patients))
	// additional stats: archived, consultations by date, avg consultation length
	archivedCount := 0
	consultsByDate := make(map[string]int)
	var totalConsultMinutes float64
	var consultCount int
	for _, patient := range patients {
		view := patientView{RegistrationNumber: patient.RegistrationNumber, Name: patient.Name, Status: patient.Status, AssignedAt: patient.AssignedAt}
		if patient.BedID != nil {
			if bed, ok := bedByID[*patient.BedID]; ok {
				n := bed.Number
				view.BedNumber = &n
				view.BedName = bed.Name
			}
		}
		if patient.ArchivedAt != nil {
			archivedCount++
		}
		if patient.ConsultedAt != nil {
			dateKey := patient.ConsultedAt.Format("2006-01-02")
			consultsByDate[dateKey]++
			if patient.AssignedAt != nil {
				diff := patient.ConsultedAt.Sub(*patient.AssignedAt).Minutes()
				if diff >= 0 {
					totalConsultMinutes += diff
					consultCount++
				}
			}
		}
		patientViews = append(patientViews, view)
	}
	// convert consultsByDate to slice
	consultsSlice := make([]map[string]any, 0, len(consultsByDate))
	for k, v := range consultsByDate {
		consultsSlice = append(consultsSlice, map[string]any{"date": k, "count": v})
	}
	avgMinutes := 0.0
	if consultCount > 0 {
		avgMinutes = totalConsultMinutes / float64(consultCount)
	}
	stats.ArchivedPatients = archivedCount
	stats.ConsultationsByDate = consultsSlice
	stats.AvgConsultationMinutes = avgMinutes
	stats.TotalConsultations = consultCount
	user, ok := a.currentUser(r)
	return statePayload{Beds: views, Patients: patientViews, Stats: stats, Authenticated: ok, Username: user.Username, Admin: user.Username == defaultUsername, ServerTime: time.Now().Format(time.RFC3339)}, nil
}

func (a *App) listBeds() ([]Bed, error) {
	var beds []Bed
	if err := a.db.Order("number asc").Find(&beds).Error; err != nil {
		return nil, err
	}
	return beds, nil
}

func (a *App) listPatients() ([]Patient, error) {
	var patients []Patient
	if err := a.db.Order("registration_number asc").Find(&patients).Error; err != nil {
		return nil, err
	}
	return patients, nil
}

func (a *App) broadcast() {
	a.broadcastEvent("state.changed", map[string]any{"reason": "update"})
}

func (a *App) broadcastEvent(event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("broadcast marshal: %v", err)
		return
	}
	message := fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)
	a.clientsMu.Lock()
	for client := range a.clients {
		select {
		case client <- message:
		default:
		}
	}
	a.clientsMu.Unlock()
}

func (a *App) lockBed(number int) func() {
	a.bedLocksMu.Lock()
	lock, ok := a.bedLocks[number]
	if !ok {
		lock = &sync.Mutex{}
		a.bedLocks[number] = lock
	}
	a.bedLocksMu.Unlock()
	lock.Lock()
	return func() {
		lock.Unlock()
	}
}

func (a *App) logBedChange(username, action string, before, after Bed) {
	oldData, _ := json.Marshal(map[string]any{
		"number":      before.Number,
		"name":        before.Name,
		"type":        before.Type,
		"status":      before.Status,
		"time":        before.Time,
		"patientName": before.PatientName,
	})
	newData, _ := json.Marshal(map[string]any{
		"number":      after.Number,
		"name":        after.Name,
		"type":        after.Type,
		"status":      after.Status,
		"time":        after.Time,
		"patientName": after.PatientName,
	})
	entityKey := ""
	if after.Number > 0 {
		entityKey = fmt.Sprintf("bed:%d", after.Number)
	} else if before.Number > 0 {
		entityKey = fmt.Sprintf("bed:%d", before.Number)
	}
	entry := AuditLog{
		Username:  fallback(username, "system"),
		Entity:    "bed",
		EntityKey: entityKey,
		Action:    action,
		OldValue:  string(oldData),
		NewValue:  string(newData),
	}
	if err := a.db.Create(&entry).Error; err != nil {
		log.Printf("audit log failed: %v", err)
	}
}

func latestBackupFile() (string, error) {
	entries, err := os.ReadDir(backupDirName)
	if err != nil {
		return "", err
	}
	paths := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "bedboard_") && strings.HasSuffix(name, ".db") {
			paths = append(paths, filepath.Join(backupDirName, name))
		}
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("no backup file")
	}
	sort.Strings(paths)
	return paths[len(paths)-1], nil
}

func restoreDatabaseFile(backupFile string) error {
	in, err := os.Open(backupFile)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dbFileName)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func (a *App) reopenDatabase() error {
	if a.db != nil {
		sqlDB, err := a.db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}
	db, err := gorm.Open(sqlite.Open(dbFileName), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&Bed{}, &Patient{}, &AdminUser{}, &Session{}, &AuditLog{}); err != nil {
		return err
	}
	a.db = db
	return nil
}

func (a *App) registerClient(ch chan string) {
	a.clientsMu.Lock()
	a.clients[ch] = struct{}{}
	a.clientsMu.Unlock()
}

func (a *App) unregisterClient(ch chan string) {
	a.clientsMu.Lock()
	if _, ok := a.clients[ch]; ok {
		delete(a.clients, ch)
		close(ch)
	}
	a.clientsMu.Unlock()
}

func (a *App) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.isAuthenticated(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (a *App) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := a.currentUser(r)
		if !ok || user.Username != defaultUsername {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func (a *App) currentUser(r *http.Request) (AdminUser, bool) {
	if r == nil {
		return AdminUser{}, false
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return AdminUser{}, false
	}
	var session Session
	if err := a.db.Where("token = ? AND expires_at > ?", cookie.Value, time.Now()).First(&session).Error; err != nil {
		return AdminUser{}, false
	}
	var user AdminUser
	if err := a.db.First(&user, session.AdminUserID).Error; err != nil {
		return AdminUser{}, false
	}
	return user, true
}

func (a *App) isAuthenticated(r *http.Request) bool {
	_, ok := a.currentUser(r)
	return ok
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "occupé", "occupe", "occupied", "busy":
		return statusOccupied
	case "nettoyage", "cleaning":
		return statusCleaning
	case "alerte", "alert":
		return statusAlert
	case "libre", "free", "available", "":
		return statusFree
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case thoracicBedType, "thoracic":
		return thoracicBedType
	default:
		return defaultBedType
	}
}

func fallback(value, def string) string {
	if strings.TrimSpace(value) == "" {
		return def
	}
	return strings.TrimSpace(value)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (a *App) withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !sameOrigin(origin, r.Host) && !isAllowedOrigin(origin) {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func (a *App) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self';")
		next.ServeHTTP(w, r)
	})
}

func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, host)
}

func isAllowedOrigin(origin string) bool {
	allow := strings.TrimSpace(os.Getenv("CORS_ALLOW_ORIGIN"))
	if allow == "" {
		return false
	}
	for _, candidate := range strings.Split(allow, ",") {
		if strings.EqualFold(strings.TrimSpace(candidate), origin) {
			return true
		}
	}
	return false
}
