package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func (a *App) collectState(r *http.Request) (statePayload, error) {
	a.dbMu.RLock()
	defer a.dbMu.RUnlock()

	beds, err := a.listBeds()
	if err != nil {
		return statePayload{}, err
	}
	patients, err := a.listPatients()
	if err != nil {
		return statePayload{}, err
	}
	user, ok := a.currentUser(r)
	role := roleOf(user)
	receptionRestricted := role == roleReception

	stats := statsView{TotalBeds: len(beds), TotalPatients: len(patients), TriageByLevel: map[string]int{"0": 0, "1": 0, "2": 0, "3": 0, "4": 0}}
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
			Room:        bed.Room,
			RoomAlt:     bed.RoomAlt,
			Name:        bed.Name,
			NameAlt:     bed.NameAlt,
			Type:        normalizeType(bed.Type),
			Status:      normalizeStatus(bed.Status),
			Time:        bed.Time,
			PatientName: bed.PatientName,
		}
		if bed.PatientID != nil {
			if patient, ok := patientByID[*bed.PatientID]; ok {
				if !receptionRestricted {
					view.PatientName = patient.Name
				}
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
		view := patientView{RegistrationNumber: patient.RegistrationNumber, Name: patient.Name, TriageScore: patient.TriageScore, Status: patient.Status, AssignedAt: patient.AssignedAt}
		if receptionRestricted {
			view.Name = ""
			view.TriageScore = 0
		}
		if patient.BedID != nil {
			if bed, ok := bedByID[*patient.BedID]; ok {
				n := bed.Number
				view.BedNumber = &n
				view.RoomName = bed.Room
				view.RoomNameAlt = bed.RoomAlt
				view.BedName = bed.Name
				view.BedNameAlt = bed.NameAlt
			}
		}
		if patient.ArchivedAt != nil {
			archivedCount++
		}
		key := strconv.Itoa(patient.TriageScore)
		stats.TriageByLevel[key]++
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
	return statePayload{Beds: views, Patients: patientViews, Stats: stats, Authenticated: ok, Username: user.Username, Admin: isAdminLike(user), ServerTime: time.Now().Format(time.RFC3339)}, nil
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

func sanitizeBackupPath(target string) (string, error) {
	if strings.TrimSpace(target) == "" {
		return "", fmt.Errorf("empty backup path")
	}
	cleanTarget := filepath.Clean(target)
	cleanBackupDir := filepath.Clean(backupDirName)

	absBackupDir, err := filepath.Abs(cleanBackupDir)
	if err != nil {
		return "", err
	}
	absTarget, err := filepath.Abs(cleanTarget)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absBackupDir, absTarget)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal detected")
	}

	return absTarget, nil
}

func restoreDatabaseFile(backupFile string) error {
	in, err := os.Open(backupFile)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dbFileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := ensurePrivateFilePermissions(dbFileName); err != nil {
		return err
	}
	return out.Sync()
}

func ensurePrivateDirPermissions(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return os.Chmod(path, 0o700)
}

func ensurePrivateFilePermissions(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return os.Chmod(path, 0o600)
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
	if err := db.AutoMigrate(&Bed{}, &Patient{}, &AdminUser{}, &Session{}, &AuditLog{}, &AppSetting{}); err != nil {
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

func (a *App) requireAuthDB(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.dbMu.RLock()
		authenticated := a.isAuthenticated(r)
		a.dbMu.RUnlock()
		if !authenticated {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (a *App) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := a.currentUser(r)
		if !ok || !isAdminLike(user) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func (a *App) requireAdminDB(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.dbMu.RLock()
		user, ok := a.currentUser(r)
		a.dbMu.RUnlock()
		if !ok || !isAdminLike(user) {
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

func (a *App) withDBRead(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.dbMu.RLock()
		defer a.dbMu.RUnlock()
		next(w, r)
	}
}

func (a *App) withDBWrite(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.dbMu.Lock()
		defer a.dbMu.Unlock()
		next(w, r)
	}
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
		if envBool("ENABLE_HSTS", false) {
			maxAge := envInt("HSTS_MAX_AGE", 31536000)
			hstsValue := fmt.Sprintf("max-age=%d", maxAge)
			if envBool("HSTS_INCLUDE_SUBDOMAINS", true) {
				hstsValue += "; includeSubDomains"
			}
			if envBool("HSTS_PRELOAD", false) {
				hstsValue += "; preload"
			}
			w.Header().Set("Strict-Transport-Security", hstsValue)
		}
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
