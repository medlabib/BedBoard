package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"
)

type latencyStats struct {
	min time.Duration
	p50 time.Duration
	p95 time.Duration
	max time.Duration
	avg time.Duration
}

func computeLatencyStats(samples []time.Duration) latencyStats {
	if len(samples) == 0 {
		return latencyStats{}
	}
	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var total time.Duration
	for _, d := range sorted {
		total += d
	}
	p50Idx := int(float64(len(sorted)-1) * 0.50)
	p95Idx := int(float64(len(sorted)-1) * 0.95)
	return latencyStats{
		min: sorted[0],
		p50: sorted[p50Idx],
		p95: sorted[p95Idx],
		max: sorted[len(sorted)-1],
		avg: time.Duration(int64(total) / int64(len(sorted))),
	}
}

func measureHandlerLatency(t *testing.T, warmup, iterations int, makeReq func(i int) *http.Request, handler http.HandlerFunc) latencyStats {
	t.Helper()
	for i := 0; i < warmup; i++ {
		rr := httptest.NewRecorder()
		handler(rr, makeReq(i))
		if rr.Code >= http.StatusBadRequest {
			t.Fatalf("warmup failed with status %d: %s", rr.Code, rr.Body.String())
		}
	}
	samples := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		rr := httptest.NewRecorder()
		start := time.Now()
		handler(rr, makeReq(i))
		samples = append(samples, time.Since(start))
		if rr.Code >= http.StatusBadRequest {
			t.Fatalf("iteration %d failed with status %d: %s", i, rr.Code, rr.Body.String())
		}
	}
	return computeLatencyStats(samples)
}

func seedLatencyDataset(t *testing.T, app *App) {
	t.Helper()
	if err := app.db.Exec("DELETE FROM beds").Error; err != nil {
		t.Fatalf("clear beds: %v", err)
	}
	if err := app.db.Exec("DELETE FROM patients").Error; err != nil {
		t.Fatalf("clear patients: %v", err)
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
		t.Fatalf("seed beds: %v", err)
	}

	patients := make([]Patient, 0, 120)
	for i := 1; i <= 120; i++ {
		arrived := now.Add(-time.Duration(i) * time.Minute)
		patients = append(patients, Patient{
			RegistrationNumber: fmt.Sprintf("P-%03d", i),
			Name:               fmt.Sprintf("Patient %d", i),
			PatientType:        patientTypeMedical,
			TriageScore:        i % 5,
			Status:             patientStatusArrived,
			ArrivedAt:          &arrived,
		})
	}
	if err := app.db.Create(&patients).Error; err != nil {
		t.Fatalf("seed patients: %v", err)
	}
}

func TestAPILatencySnapshot(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()
	seedLatencyDataset(t, app)

	token := createAdminSession(t, app)
	stateHandler := app.withCORS(app.requireAuthDB(app.withDBRead(app.handleState)))
	patientsHandler := app.withCORS(app.requireAuthDB(app.withDBWrite(app.handlePatients)))

	stateStats := measureHandlerLatency(t, 20, 300, func(i int) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		return req
	}, stateHandler)

	payloadA := `{"registrationNumber":"PERF-API-1","name":"Perf User","patientType":"medical","triageScore":2,"reason":"test"}`
	payloadB := `{"registrationNumber":"PERF-API-1","name":"Perf User","patientType":"medical","triageScore":3,"reason":"test"}`
	patientsStats := measureHandlerLatency(t, 20, 300, func(i int) *http.Request {
		body := payloadA
		if i%2 == 1 {
			body = payloadB
		}
		req := httptest.NewRequest(http.MethodPost, "/api/patients", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		return req
	}, patientsHandler)

	t.Logf("/api/state latency: min=%s p50=%s p95=%s max=%s avg=%s", stateStats.min, stateStats.p50, stateStats.p95, stateStats.max, stateStats.avg)
	t.Logf("/api/patients POST latency: min=%s p50=%s p95=%s max=%s avg=%s", patientsStats.min, patientsStats.p50, patientsStats.p95, patientsStats.max, patientsStats.avg)
}
