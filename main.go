package main

import (
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

//go:embed frontend/dist icon.svg logo.png
var embeddedFiles embed.FS

const (
	defaultPort       = ":8080"
	dbFileName        = "bedboard.db"
	legacyConfigFile  = "config_salle.json"
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
	ID                 uint      `gorm:"primaryKey" json:"-"`
	RegistrationNumber string    `gorm:"uniqueIndex;not null" json:"registrationNumber"`
	Name               string    `json:"name"`
	BedID              *uint     `json:"-"`
	Status             string    `json:"status"`
	AssignedAt         *time.Time `json:"assignedAt"`
	ConsultedAt        *time.Time `json:"consultedAt"`
	ArchivedAt         *time.Time `json:"archivedAt"`
	ExitAt             *time.Time `json:"exitAt"`
	CreatedAt          time.Time `json:"-"`
	UpdatedAt          time.Time `json:"-"`
}

type AdminUser struct {
	ID           uint      `gorm:"primaryKey" json:"-"`
	Username     string    `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"-"`
	UpdatedAt    time.Time `json:"-"`
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
	db        *gorm.DB
	clientsMu sync.Mutex
	clients   map[chan string]struct{}
}

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
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
	RegistrationNumber string `json:"registrationNumber"`
	Name               string `json:"name"`
	BedNumber          *int   `json:"bedNumber"`
	BedName            string `json:"bedName"`
	Status             string `json:"status"`
}

type statsView struct {
	TotalBeds     int `json:"totalBeds"`
	FreeBeds      int `json:"freeBeds"`
	OccupiedBeds  int `json:"occupiedBeds"`
	CleaningBeds  int `json:"cleaningBeds"`
	AlertBeds     int `json:"alertBeds"`
	TotalPatients int `json:"totalPatients"`
	ArchivedPatients int                    `json:"archivedPatients"`
	ConsultationsByDate []map[string]any    `json:"consultationsByDate"`
	AvgConsultationMinutes float64          `json:"avgConsultationMinutes"`
	TotalConsultations int                    `json:"totalConsultations"`
}

func main() {
	app := &App{clients: make(map[chan string]struct{})}
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
	mux.HandleFunc("/icon.svg", func(w http.ResponseWriter, r *http.Request) {
		iconFile, err := fs.ReadFile(embeddedFiles, "icon.svg")
		if err != nil {
			http.Error(w, "icon not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write(iconFile)
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
	mux.HandleFunc("/api/stream", app.withCORS(app.handleStream))
	mux.HandleFunc("/api/users", app.withCORS(app.requireAuth(app.requireAdmin(app.handleUsers))))
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

	log.Printf("BedBoard listening on http://localhost%s", serverAddr)
	if err := http.ListenAndServe(serverAddr, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	if err := db.AutoMigrate(&Bed{}, &Patient{}, &AdminUser{}, &Session{}); err != nil {
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
		if err := a.db.Create(&AdminUser{Username: username, PasswordHash: string(hash)}).Error; err != nil {
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

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := a.currentUser(r)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": ok, "username": user.Username, "admin": user.Username == defaultUsername})
}

func (a *App) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
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
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
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
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
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
		user := AdminUser{Username: username, PasswordHash: string(hash)}
		if err := a.db.Create(&user).Error; err != nil {
			http.Error(w, "create failed", http.StatusInternalServerError)
			return
		}
		a.broadcast()
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "user": userView{Username: user.Username, Admin: false}})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
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
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
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

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
	bed.Status = normalizeStatus(req.Status)
	if req.PatientName != "" {
		bed.PatientName = req.PatientName
	}
	if bed.Status == statusFree || bed.Status == statusCleaning || bed.Status == statusAlert {
		if bed.PatientID != nil {
			a.releasePatientByID(*bed.PatientID)
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
	a.broadcast()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleConfigBed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
	a.broadcast()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleBedsCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
	a.broadcast()
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (a *App) handleBedsDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
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
	if bed.PatientID != nil {
		a.releasePatientByID(*bed.PatientID)
	}
	if err := a.db.Delete(&bed).Error; err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	a.broadcast()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handlePatients(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req patientRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.RegistrationNumber) == "" || strings.TrimSpace(req.Name) == "" {
			http.Error(w, "registration number and name required", http.StatusBadRequest)
			return
		}
		patient, err := a.upsertAndAssignPatient(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.broadcast()
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
	var req struct{
		RegistrationNumber string `json:"registrationNumber"`
		Action string `json:"action"`
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
	a.broadcast()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "patient": patient})
}

func (a *App) upsertAndAssignPatient(req patientRequest) (Patient, error) {
	bedNumber := req.BedNumber
	if bedNumber == 0 {
		bedNumber = req.BedID
	}
	var patient Patient
	if err := a.db.Where("registration_number = ?", strings.TrimSpace(req.RegistrationNumber)).First(&patient).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return Patient{}, err
		}
		patient = Patient{RegistrationNumber: strings.TrimSpace(req.RegistrationNumber)}
	}
	patient.Name = strings.TrimSpace(req.Name)
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
	if bed.PatientID != nil && *bed.PatientID != patient.ID {
		var prevPatient Patient
		if err := a.db.First(&prevPatient, *bed.PatientID).Error; err == nil {
			prevPatient.BedID = nil
			_ = a.db.Save(&prevPatient).Error
		}
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
	return patient, nil
}

func (a *App) releasePatientByID(patientID uint) {
	var patient Patient
	if err := a.db.First(&patient, patientID).Error; err != nil {
		return
	}
	if patient.BedID != nil {
		var bed Bed
		if err := a.db.First(&bed, *patient.BedID).Error; err == nil {
			bed.PatientID = nil
			bed.PatientName = ""
			bed.Status = statusFree
			_ = a.db.Save(&bed).Error
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
		view := patientView{RegistrationNumber: patient.RegistrationNumber, Name: patient.Name, Status: patient.Status}
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
	state, err := a.collectState(nil)
	if err != nil {
		log.Printf("broadcast state: %v", err)
		return
	}
	data, err := json.Marshal(state)
	if err != nil {
		log.Printf("broadcast marshal: %v", err)
		return
	}
	message := fmt.Sprintf("data: %s\n\n", data)
	a.clientsMu.Lock()
	for client := range a.clients {
		select {
		case client <- message:
		default:
		}
	}
	a.clientsMu.Unlock()
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
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}
