package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (a *App) handleFHIRExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a.dbMu.RLock()
	patients, err := a.listPatients()
	beds, _ := a.listBeds()
	a.dbMu.RUnlock()
	if err != nil {
		http.Error(w, "patient export failed", http.StatusInternalServerError)
		return
	}

	includeArchived := strings.TrimSpace(r.URL.Query().Get("includeArchived")) == "1"
	bedByNumber := map[uint]Bed{}
	for _, bed := range beds {
		bedByNumber[bed.ID] = bed
	}

	entries := make([]map[string]any, 0, len(patients)*2)
	for _, patient := range patients {
		status := strings.TrimSpace(strings.ToLower(patient.Status))
		if !includeArchived && (status == patientStatusArchived || status == patientStatusConsulted) {
			continue
		}

		patientResource := map[string]any{
			"resourceType": "Patient",
			"id":           patient.RegistrationNumber,
			"identifier": []map[string]any{
				{
					"system": "urn:bedboard:registration",
					"value":  patient.RegistrationNumber,
				},
			},
			"name": []map[string]any{{"text": patient.Name}},
			"extension": []map[string]any{
				{"url": "urn:bedboard:patient-type", "valueString": patient.PatientType},
				{"url": "urn:bedboard:triage-score", "valueInteger": patient.TriageScore},
				{"url": "urn:bedboard:status", "valueString": patient.Status},
			},
		}
		entries = append(entries, map[string]any{"resource": patientResource})

		encounterStatus := "in-progress"
		if status == patientStatusConsulted || status == patientStatusArchived || status == patientStatusTransferred || status == patientStatusDeceased {
			encounterStatus = "finished"
		}

		encounter := map[string]any{
			"resourceType": "Encounter",
			"id":           fmt.Sprintf("enc-%s", patient.RegistrationNumber),
			"status":       encounterStatus,
			"subject": map[string]any{
				"reference": "Patient/" + patient.RegistrationNumber,
			},
			"period": map[string]any{},
		}
		period := encounter["period"].(map[string]any)
		if patient.ArrivedAt != nil {
			period["start"] = patient.ArrivedAt.Format(time.RFC3339)
		}
		if patient.ExitAt != nil {
			period["end"] = patient.ExitAt.Format(time.RFC3339)
		}
		if patient.BedID != nil {
			if bed, ok := bedByNumber[*patient.BedID]; ok {
				encounter["location"] = []map[string]any{{
					"location": map[string]any{"display": strings.TrimSpace(fmt.Sprintf("%s - %s", bed.Room, bed.Name))},
				}}
			}
		}
		entries = append(entries, map[string]any{"resource": encounter})
	}

	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "collection",
		"timestamp":    time.Now().Format(time.RFC3339),
		"total":        len(entries),
		"entry":        entries,
	}

	filename := "bedboard-fhir-" + strconv.FormatInt(time.Now().Unix(), 10) + ".json"
	w.Header().Set("Content-Type", "application/fhir+json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(bundle)
}
