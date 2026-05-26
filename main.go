package main

import (
	"embed"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

//go:embed frontend/dist logo.svg
var embeddedFiles embed.FS

const (
	defaultPort       = ":8080"
	dbFileName        = "bedboard.db"
	backupDirName     = "backups"
	sessionCookieName = "bedboard_session"
	sessionDuration   = 7 * 24 * time.Hour
	defaultUsername   = "admin"
	defaultBedType    = "standard"
	thoracicBedType   = "thoracique"
	statusFree        = "libre"
	statusOccupied    = "occupé"
	statusCleaning    = "nettoyage"
	statusAlert       = "alerte"
	roleAdmin         = "admin"
	roleUser          = "user"
	roleReception     = "reception"
	roleTriage        = "triage"
	roleDechocage     = "dechocage"
)

type Bed struct {
	ID          uint      `gorm:"primaryKey" json:"-"`
	Number      int       `gorm:"uniqueIndex;not null" json:"number"`
	Room        string    `json:"room"`
	RoomAlt     string    `json:"roomAlt"`
	Name        string    `json:"name"`
	NameAlt     string    `json:"nameAlt"`
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
	TriageScore        int        `json:"triageScore"`
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
	Role              string     `gorm:"index;not null;default:user" json:"role"`
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

type AppSetting struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	Key       string    `gorm:"uniqueIndex;size:128;not null" json:"key"`
	Value     string    `gorm:"type:text;not null;default:''" json:"value"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

type App struct {
	db            *gorm.DB
	dbMu          sync.RWMutex
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
	Role     string `json:"role"`
}

type passwordChangeRequest struct {
	Username        string `json:"username"`
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type restoreRequest struct {
	File string `json:"file"`
}

type gotifySettingsRequest struct {
	Enabled    bool   `json:"enabled"`
	URL        string `json:"url"`
	Token      string `json:"token"`
	Priority   int    `json:"priority"`
	ClearToken bool   `json:"clearToken"`
}

type gotifySettingsView struct {
	Enabled         bool   `json:"enabled"`
	URL             string `json:"url"`
	Priority        int    `json:"priority"`
	TokenConfigured bool   `json:"tokenConfigured"`
}

type userView struct {
	Username string `json:"username"`
	Admin    bool   `json:"admin"`
	Role     string `json:"role"`
}

type statusRequest struct {
	Number      int    `json:"number"`
	ID          int    `json:"id"`
	Status      string `json:"status"`
	PatientName string `json:"patientName"`
}

type bedRequest struct {
	Number  int    `json:"number"`
	Room    string `json:"room"`
	RoomAlt string `json:"roomAlt"`
	Name    string `json:"name"`
	NameAlt string `json:"nameAlt"`
	Type    string `json:"type"`
}

type patientRequest struct {
	RegistrationNumber string `json:"registrationNumber"`
	Name               string `json:"name"`
	BedNumber          int    `json:"bedNumber"`
	BedID              int    `json:"bedId"`
	TriageScore        *int   `json:"triageScore"`
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
	Room                string `json:"room"`
	RoomAlt             string `json:"roomAlt"`
	Name                string `json:"name"`
	NameAlt             string `json:"nameAlt"`
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
	TriageScore        int        `json:"triageScore"`
	BedNumber          *int       `json:"bedNumber"`
	RoomName           string     `json:"roomName"`
	RoomNameAlt        string     `json:"roomNameAlt"`
	BedName            string     `json:"bedName"`
	BedNameAlt         string     `json:"bedNameAlt"`
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
	TriageByLevel          map[string]int   `json:"triageByLevel"`
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
	mux.HandleFunc("/logo.svg", func(w http.ResponseWriter, r *http.Request) {
		logoFile, err := fs.ReadFile(embeddedFiles, "logo.svg")
		if err != nil {
			http.Error(w, "logo not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write(logoFile)
	})
	mux.HandleFunc("/api/me", app.withCORS(app.withDBRead(app.handleMe)))
	mux.HandleFunc("/api/auth", app.withCORS(app.withDBWrite(app.handleAuth)))
	mux.HandleFunc("/api/logout", app.withCORS(app.withDBWrite(app.handleLogout)))
	mux.HandleFunc("/api/state", app.withCORS(app.requireAuthDB(app.withDBRead(app.handleState))))
	mux.HandleFunc("/api/stream", app.withCORS(app.requireAuthDB(app.handleStream)))
	mux.HandleFunc("/api/users", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBWrite(app.handleUsers)))))
	mux.HandleFunc("/api/users/password", app.withCORS(app.requireAuthDB(app.withDBWrite(app.handleUserPassword))))
	mux.HandleFunc("/api/audit", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBRead(app.handleAudit)))))
	mux.HandleFunc("/api/admin/security/health", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBRead(app.handleSecurityHealth)))))
	mux.HandleFunc("/api/admin/backup", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBWrite(app.handleBackup)))))
	mux.HandleFunc("/api/admin/restore", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBWrite(app.handleRestore)))))
	mux.HandleFunc("/api/admin/integrations/gotify", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBWrite(app.handleGotifySettings)))))
	mux.HandleFunc("/api/status", app.withCORS(app.requireAuthDB(app.withDBWrite(app.handleStatus))))
	mux.HandleFunc("/api/config-bed", app.withCORS(app.requireAuthDB(app.withDBWrite(app.handleConfigBed))))
	mux.HandleFunc("/api/beds", app.withCORS(app.requireAuthDB(app.withDBWrite(app.handleBedsCreate))))
	mux.HandleFunc("/api/beds/delete", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBWrite(app.handleBedsDelete)))))
	mux.HandleFunc("/api/patients", app.withCORS(app.requireAuthDB(app.withDBWrite(app.handlePatients))))
	mux.HandleFunc("/api/patients/archive", app.withCORS(app.requireAuthDB(app.withDBWrite(app.handlePatientsArchive))))

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
