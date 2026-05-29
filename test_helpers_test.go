package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func setupTestApp(t *testing.T) (*App, func()) {
	t.Helper()
	oldDataDir := dataDirPath
	oldDB := dbFileName
	oldBackup := backupDirName

	baseDir := t.TempDir()
	if err := useDataDir(baseDir); err != nil {
		t.Fatalf("useDataDir: %v", err)
	}

	app := &App{clients: make(map[chan string]struct{}), bedLocks: make(map[int]*sync.Mutex)}
	if err := app.initDatabase(); err != nil {
		t.Fatalf("initDatabase: %v", err)
	}

	cleanup := func() {
		if app.db != nil {
			sqlDB, err := app.db.DB()
			if err == nil {
				_ = sqlDB.Close()
			}
		}
		dataDirPath = oldDataDir
		dbFileName = oldDB
		backupDirName = oldBackup
	}

	return app, cleanup
}

func mustJSONBody(t *testing.T, payload any) *bytes.Reader {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	return bytes.NewReader(raw)
}

func createAdminSession(t *testing.T, app *App) string {
	t.Helper()
	var admin AdminUser
	if err := app.db.Where("username = ?", defaultUsername).First(&admin).Error; err != nil {
		t.Fatalf("load admin user: %v", err)
	}
	token := "test-session-token"
	if err := app.db.Create(&Session{
		Token:       token,
		AdminUserID: admin.ID,
		ExpiresAt:   time.Now().Add(2 * time.Hour),
	}).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}
	return token
}

func newAuthedJSONRequest(t *testing.T, method, path, token string, payload any) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, mustJSONBody(t, payload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	return req
}
