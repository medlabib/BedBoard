package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func intPtr(v int) *int {
	return &v
}

func TestPatientAssignReleaseLifecycle(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	patient, err := app.upsertAndAssignPatient(patientRequest{
		RegistrationNumber: "REG-100",
		Name:               "PATIENT-A",
		PatientType:        patientTypeMedical,
		TriageScore:        intPtr(4),
		BedNumber:          1,
		Reason:             "test_reason",
		Destination:        "test_destination",
		Outcome:            "test_outcome",
	}, "admin")
	if err != nil {
		t.Fatalf("upsertAndAssignPatient: %v", err)
	}

	if patient.Status != patientStatusAssigned {
		t.Fatalf("expected status %q, got %q", patientStatusAssigned, patient.Status)
	}
	if patient.ArrivedAt == nil || patient.TriagedAt == nil || patient.AssignedAt == nil || patient.StartedAt == nil {
		t.Fatalf("expected lifecycle timestamps to be set")
	}

	app.releasePatientByID(patient.ID, "admin", true)

	var reloaded Patient
	if err := app.db.Where("registration_number = ?", "REG-100").First(&reloaded).Error; err != nil {
		t.Fatalf("reload patient: %v", err)
	}
	if reloaded.BedID != nil {
		t.Fatalf("expected patient bed to be released")
	}
	if reloaded.ConsultedAt == nil {
		t.Fatalf("expected consultedAt to be set on release")
	}
	if reloaded.Status != patientStatusConsulted {
		t.Fatalf("expected consulted status, got %q", reloaded.Status)
	}

	var assignedEvents int64
	if err := app.db.Model(&PatientEvent{}).Where("registration_number = ? AND event = ?", "REG-100", "patient.assigned").Count(&assignedEvents).Error; err != nil {
		t.Fatalf("count assigned events: %v", err)
	}
	if assignedEvents == 0 {
		t.Fatalf("expected patient.assigned event")
	}

	var releasedEvents int64
	if err := app.db.Model(&PatientEvent{}).Where("registration_number = ? AND event = ?", "REG-100", "patient.released").Count(&releasedEvents).Error; err != nil {
		t.Fatalf("count released events: %v", err)
	}
	if releasedEvents == 0 {
		t.Fatalf("expected patient.released event")
	}
}

func TestCollectStateIncludesSLAAndOperationalMetrics(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	if err := app.upsertSettingValue(settingTriageSLAMinutes, "10"); err != nil {
		t.Fatalf("set triage SLA: %v", err)
	}

	arrivedOld := time.Now().Add(-30 * time.Minute)
	if err := app.db.Create(&Patient{
		RegistrationNumber: "REG-SLA",
		Name:               "PATIENT-SLA",
		PatientType:        patientTypeMedical,
		TriageScore:        4,
		Status:             patientStatusTriaged,
		ArrivedAt:          &arrivedOld,
	}).Error; err != nil {
		t.Fatalf("create patient: %v", err)
	}

	state, err := app.collectState(httptest.NewRequest(http.MethodGet, "/api/state", nil))
	if err != nil {
		t.Fatalf("collectState: %v", err)
	}

	if state.Stats.TriageSLABreaches != 1 {
		t.Fatalf("expected 1 SLA breach, got %d", state.Stats.TriageSLABreaches)
	}
	if state.Stats.PatientsByStatus[patientStatusTriaged] != 1 {
		t.Fatalf("expected patient status count for triaged")
	}
	if state.Stats.PatientsByType[patientTypeMedical] != 1 {
		t.Fatalf("expected patient type count for medical")
	}
}

func TestHandlePatientsImportCreatesPatientsAndEvents(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	token := createAdminSession(t, app)

	req := newAuthedJSONRequest(t, http.MethodPost, "/api/admin/integrations/patients/import", token, map[string]any{
		"source": "sih",
		"patients": []map[string]any{
			{"registrationNumber": "IMP-1", "name": "PATIENT-IMPORT-1", "patientType": patientTypeMedical, "triageScore": 2},
			{"registrationNumber": "IMP-2", "name": "PATIENT-IMPORT-2", "patientType": patientTypeTraumato, "triageScore": 3},
		},
	})

	rr := httptest.NewRecorder()
	app.handlePatientsImport(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if int(body["processed"].(float64)) != 2 {
		t.Fatalf("expected processed=2, got %v", body["processed"])
	}

	var count int64
	if err := app.db.Model(&Patient{}).Where("registration_number IN ?", []string{"IMP-1", "IMP-2"}).Count(&count).Error; err != nil {
		t.Fatalf("count imported patients: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 imported patients, got %d", count)
	}

	var importedEvents int64
	if err := app.db.Model(&PatientEvent{}).Where("event = ?", "patient.imported").Count(&importedEvents).Error; err != nil {
		t.Fatalf("count imported events: %v", err)
	}
	if importedEvents < 2 {
		t.Fatalf("expected at least 2 patient.imported events, got %d", importedEvents)
	}
}

func TestHandleAuditExportReturnsCSV(t *testing.T) {
	app, cleanup := setupTestApp(t)
	defer cleanup()

	app.logBedChange("admin", "bed.test", Bed{Number: 1, Status: statusFree}, Bed{Number: 1, Status: statusOccupied})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/audit/export", nil)
	rr := httptest.NewRecorder()
	app.handleAuditExport(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "text/csv") {
		t.Fatalf("expected CSV content type, got %q", rr.Header().Get("Content-Type"))
	}
	content := rr.Body.String()
	if !strings.Contains(content, "createdAt,username,entity,entityKey,action,oldValue,newValue") {
		t.Fatalf("expected CSV header")
	}
	if !strings.Contains(content, "bed.test") {
		t.Fatalf("expected audit action in CSV export")
	}
}
