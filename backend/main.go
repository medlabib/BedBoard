package main

import (
	"embed"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

//go:embed frontend/dist logo.svg
var embeddedFiles embed.FS

const (
	defaultPort                      = ":8080"
	defaultDataDirName               = "BedBoard"
	defaultDBFileBaseName            = "bedboard.db"
	defaultBackupDirBaseName         = "backups"
	insecureDefaultBootstrapPassword = "ChangeMe!123"
	sessionCookieName                = "bedboard_session"
	sessionDuration                  = 7 * 24 * time.Hour
	defaultUsername                  = "admin"
	defaultBedType                   = "standard"
	thoracicBedType                  = "thoracique"
	statusFree                       = "libre"
	statusOccupied                   = "occupé"
	statusCleaning                   = "nettoyage"
	statusAlert                      = "alerte"
	roleAdmin                        = "admin"
	roleUser                         = "user"
	roleReception                    = "reception"
	roleTriage                       = "triage"
	roleDechocage                    = "dechocage"
	patientTypeTraumato              = "traumato"
	patientTypeMedical               = "medical"
	patientTypeChestPain             = "douleurs_thoracique"
	patientTypeSurgical              = "chirurgical"
	patientStatusArrived             = "arrived"
	patientStatusTriaged             = "triaged"
	patientStatusWaiting             = "waiting"
	patientStatusAssigned            = "assigned"
	patientStatusConsulted           = "consulted"
	patientStatusArchived            = "archived"
	patientStatusTransferred         = "transferred"
	patientStatusDeceased            = "deceased"
)

var (
	dataDirPath   string
	dbFileName    string
	backupDirName string
)

func resolveDataPaths() error {
	if dbFileName != "" && backupDirName != "" {
		return nil
	}

	baseDir := strings.TrimSpace(os.Getenv("BEDBOARD_DATA_DIR"))
	if baseDir != "" {
		return useDataDir(baseDir)
	}

	candidates := make([]string, 0, 6)
	if runtime.GOOS == "windows" {
		if localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); localAppData != "" {
			candidates = append(candidates, filepath.Join(localAppData, defaultDataDirName))
		}
		if configDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(configDir) != "" {
			candidates = append(candidates, filepath.Join(configDir, defaultDataDirName))
		}
		if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
			candidates = append(candidates, filepath.Join(appData, defaultDataDirName))
		}
	}

	if wd, err := os.Getwd(); err == nil && strings.TrimSpace(wd) != "" {
		candidates = append(candidates, filepath.Join(wd, defaultDataDirName))
	}
	if exePath, err := os.Executable(); err == nil && strings.TrimSpace(exePath) != "" {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates, filepath.Join(exeDir, defaultDataDirName))
	}
	candidates = append(candidates, filepath.Join(os.TempDir(), defaultDataDirName))

	var lastErr error
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if err := useDataDir(candidate); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	if lastErr != nil {
		return lastErr
	}
	return errors.New("unable to resolve writable data directory")
}

func useDataDir(baseDir string) error {
	cleanBaseDir := filepath.Clean(baseDir)
	if err := os.MkdirAll(cleanBaseDir, 0o700); err != nil {
		return err
	}
	if err := ensurePrivateDirPermissions(cleanBaseDir); err != nil {
		return err
	}
	if err := ensureDirWritable(cleanBaseDir); err != nil {
		return err
	}

	backupDir := filepath.Join(cleanBaseDir, defaultBackupDirBaseName)
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return err
	}
	if err := ensurePrivateDirPermissions(backupDir); err != nil {
		return err
	}

	dataDirPath = cleanBaseDir
	dbFileName = filepath.Join(cleanBaseDir, defaultDBFileBaseName)
	backupDirName = backupDir
	return nil
}

func ensureDirWritable(path string) error {
	testFile := filepath.Join(path, ".bedboard_write_test")
	f, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString("ok"); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	_ = os.Remove(testFile)
	return nil
}

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
	PatientType        string     `gorm:"index;not null;default:medical" json:"patientType"`
	TriageScore        int        `json:"triageScore"`
	BedID              *uint      `json:"-"`
	Status             string     `json:"status"`
	Reason             string     `json:"reason"`
	Destination        string     `json:"destination"`
	Outcome            string     `json:"outcome"`
	ArrivedAt          *time.Time `json:"arrivedAt"`
	TriagedAt          *time.Time `json:"triagedAt"`
	StartedAt          *time.Time `json:"startedAt"`
	AssignedAt         *time.Time `json:"assignedAt"`
	ConsultedAt        *time.Time `json:"consultedAt"`
	ArchivedAt         *time.Time `json:"archivedAt"`
	ExitAt             *time.Time `json:"exitAt"`
	CreatedAt          time.Time  `json:"-"`
	UpdatedAt          time.Time  `json:"-"`
}

type PatientEvent struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	RegistrationNumber string    `gorm:"index;not null" json:"registrationNumber"`
	Username           string    `json:"username"`
	Event              string    `gorm:"index;not null" json:"event"`
	Details            string    `gorm:"type:text;not null;default:''" json:"details"`
	CreatedAt          time.Time `gorm:"index" json:"createdAt"`
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
	PatientType        string `json:"patientType"`
	BedNumber          int    `json:"bedNumber"`
	BedID              int    `json:"bedId"`
	TriageScore        *int   `json:"triageScore"`
	Status             string `json:"status"`
	Reason             string `json:"reason"`
	Destination        string `json:"destination"`
	Outcome            string `json:"outcome"`
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
	PatientType        string     `json:"patientType"`
	TriageScore        int        `json:"triageScore"`
	Reason             string     `json:"reason"`
	Destination        string     `json:"destination"`
	Outcome            string     `json:"outcome"`
	BedNumber          *int       `json:"bedNumber"`
	RoomName           string     `json:"roomName"`
	RoomNameAlt        string     `json:"roomNameAlt"`
	BedName            string     `json:"bedName"`
	BedNameAlt         string     `json:"bedNameAlt"`
	Status             string     `json:"status"`
	ArrivedAt          *time.Time `json:"arrivedAt"`
	TriagedAt          *time.Time `json:"triagedAt"`
	StartedAt          *time.Time `json:"startedAt"`
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
	ConsultationsByHour    []map[string]any `json:"consultationsByHour"`
	AvgConsultationMinutes float64          `json:"avgConsultationMinutes"`
	AvgWaitToTriageMinutes float64          `json:"avgWaitToTriageMinutes"`
	AvgWaitToAssignMinutes float64          `json:"avgWaitToAssignMinutes"`
	TotalConsultations     int              `json:"totalConsultations"`
	TriageByLevel          map[string]int   `json:"triageByLevel"`
	PatientsByStatus       map[string]int   `json:"patientsByStatus"`
	PatientsByType         map[string]int   `json:"patientsByType"`
	TriageSLABreaches      int              `json:"triageSlaBreaches"`
}

func main() {
	if err := resolveDataPaths(); err != nil {
		log.Fatalf("resolve data paths: %v", err)
	}
	log.Printf("BedBoard data directory: %s", dataDirPath)

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
	mux.HandleFunc("/api/public/ui-config", app.withCORS(app.withDBRead(app.handlePublicUIConfig)))
	mux.HandleFunc("/api/state", app.withCORS(app.requireAuthDB(app.withDBRead(app.handleState))))
	mux.HandleFunc("/api/stream", app.withCORS(app.requireAuthDB(app.handleStream)))
	mux.HandleFunc("/api/users", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBWrite(app.handleUsers)))))
	mux.HandleFunc("/api/users/password", app.withCORS(app.requireAuthDB(app.withDBWrite(app.handleUserPassword))))
	mux.HandleFunc("/api/audit", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBRead(app.handleAudit)))))
	mux.HandleFunc("/api/admin/audit/export", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBRead(app.handleAuditExport)))))
	mux.HandleFunc("/api/admin/ui/config", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBWrite(app.handleUIConfig)))))
	mux.HandleFunc("/api/admin/security/health", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBRead(app.handleSecurityHealth)))))
	mux.HandleFunc("/api/admin/security/config", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBWrite(app.handleSecurityConfig)))))
	mux.HandleFunc("/api/admin/backup", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBWrite(app.handleBackup)))))
	mux.HandleFunc("/api/admin/restore", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBWrite(app.handleRestore)))))
	mux.HandleFunc("/api/admin/integrations/gotify", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBWrite(app.handleGotifySettings)))))
	mux.HandleFunc("/api/admin/integrations/gotify/test", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBWrite(app.handleGotifyTest)))))
	mux.HandleFunc("/api/admin/integrations/patients/import", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBWrite(app.handlePatientsImport)))))
	mux.HandleFunc("/api/status", app.withCORS(app.requireAuthDB(app.withDBWrite(app.handleStatus))))
	mux.HandleFunc("/api/config-bed", app.withCORS(app.requireAuthDB(app.withDBWrite(app.handleConfigBed))))
	mux.HandleFunc("/api/beds", app.withCORS(app.requireAuthDB(app.withDBWrite(app.handleBedsCreate))))
	mux.HandleFunc("/api/beds/delete", app.withCORS(app.requireAuthDB(app.requireAdminDB(app.withDBWrite(app.handleBedsDelete)))))
	mux.HandleFunc("/api/patients", app.withCORS(app.requireAuthDB(app.withDBWrite(app.handlePatients))))
	mux.HandleFunc("/api/patients/events", app.withCORS(app.requireAuthDB(app.withDBRead(app.handlePatientEvents))))
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
