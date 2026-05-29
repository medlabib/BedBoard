package main

import (
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func setupBenchmarkApp(b *testing.B) (*App, func()) {
	b.Helper()
	oldDataDir := dataDirPath
	oldDB := dbFileName
	oldBackup := backupDirName

	baseDir := b.TempDir()
	if err := useDataDir(baseDir); err != nil {
		b.Fatalf("useDataDir: %v", err)
	}

	app := &App{clients: make(map[chan string]struct{}), bedLocks: make(map[int]*sync.Mutex)}
	if err := app.initDatabase(); err != nil {
		b.Fatalf("initDatabase: %v", err)
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

func BenchmarkNormalizeStatus(b *testing.B) {
	inputs := []string{"occupied", "cleaning", "free", "alerte", "unknown"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = normalizeStatus(inputs[i%len(inputs)])
	}
}

func BenchmarkFallback(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = fallback("  value  ", "default")
	}
}

func BenchmarkCollectState(b *testing.B) {
	app, cleanup := setupBenchmarkApp(b)
	defer cleanup()
	if err := app.db.Exec("DELETE FROM beds").Error; err != nil {
		b.Fatalf("clear beds: %v", err)
	}
	if err := app.db.Exec("DELETE FROM patients").Error; err != nil {
		b.Fatalf("clear patients: %v", err)
	}

	now := time.Now()
	beds := make([]Bed, 0, 40)
	for i := 1; i <= 40; i++ {
		status := statusFree
		if i%3 == 0 {
			status = statusOccupied
		} else if i%5 == 0 {
			status = statusCleaning
		}
		beds = append(beds, Bed{
			Number: i,
			Room:   fmt.Sprintf("Room %d", (i-1)/4+1),
			Name:   fmt.Sprintf("Bed %d", i),
			Type:   defaultBedType,
			Status: status,
		})
	}
	if err := app.db.Create(&beds).Error; err != nil {
		b.Fatalf("seed beds: %v", err)
	}

	patients := make([]Patient, 0, 120)
	for i := 1; i <= 120; i++ {
		arrived := now.Add(-time.Duration(i) * time.Minute)
		patients = append(patients, Patient{
			RegistrationNumber: fmt.Sprintf("P-%03d", i),
			Name:               fmt.Sprintf("Patient %d", i),
			PatientType:        patientTypeMedical,
			Status:             patientStatusArrived,
			ArrivedAt:          &arrived,
		})
	}
	if err := app.db.Create(&patients).Error; err != nil {
		b.Fatalf("seed patients: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/state", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := app.collectState(req)
		if err != nil {
			b.Fatalf("collectState: %v", err)
		}
	}
}
