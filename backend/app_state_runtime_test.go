package main

import (
	"os"
	"strings"
	"testing"
)

func TestNormalizeHelpersAndFallbacks(t *testing.T) {
	if got := normalizeStatus("occupied"); got != statusOccupied {
		t.Fatalf("normalizeStatus occupied: got %q", got)
	}
	if got := normalizeStatus("cleaning"); got != statusCleaning {
		t.Fatalf("normalizeStatus cleaning: got %q", got)
	}
	if got := normalizeStatus("free"); got != statusFree {
		t.Fatalf("normalizeStatus free: got %q", got)
	}
	if got := normalizeType("thoracic"); got != thoracicBedType {
		t.Fatalf("normalizeType thoracic: got %q", got)
	}
	if got := normalizeType("whatever"); got != defaultBedType {
		t.Fatalf("normalizeType fallback: got %q", got)
	}
	if got := normalizePatientType("chest_pain"); got != patientTypeChestPain {
		t.Fatalf("normalizePatientType chest_pain: got %q", got)
	}
	if got := normalizePatientType("unknown"); got != "" {
		t.Fatalf("normalizePatientType unknown should be empty, got %q", got)
	}
	if got := fallback("  ", "x"); got != "x" {
		t.Fatalf("fallback default: got %q", got)
	}
	if got := fallback("  ok ", "x"); got != "ok" {
		t.Fatalf("fallback trim: got %q", got)
	}
}

func TestRandomTokenAndOriginChecks(t *testing.T) {
	t1, err := randomToken()
	if err != nil {
		t.Fatalf("randomToken 1: %v", err)
	}
	t2, err := randomToken()
	if err != nil {
		t.Fatalf("randomToken 2: %v", err)
	}
	if t1 == "" || t2 == "" || t1 == t2 {
		t.Fatalf("expected two distinct non-empty tokens")
	}

	if !sameOrigin("https://example.com", "example.com") {
		t.Fatalf("sameOrigin should match host")
	}
	if sameOrigin("https://evil.com", "example.com") {
		t.Fatalf("sameOrigin should not match different host")
	}
	if sameOrigin("not-a-url", "example.com") {
		t.Fatalf("sameOrigin should reject invalid URL")
	}
}

func TestIsAllowedOriginFromEnv(t *testing.T) {
	old := os.Getenv("CORS_ALLOW_ORIGIN")
	t.Cleanup(func() {
		_ = os.Setenv("CORS_ALLOW_ORIGIN", old)
	})

	_ = os.Setenv("CORS_ALLOW_ORIGIN", "https://a.example.com, https://b.example.com")
	if !isAllowedOrigin("https://a.example.com") {
		t.Fatalf("expected allowed origin")
	}
	if !isAllowedOrigin("https://b.example.com") {
		t.Fatalf("expected second allowed origin")
	}
	if isAllowedOrigin("https://c.example.com") {
		t.Fatalf("did not expect unknown origin")
	}

	_ = os.Setenv("CORS_ALLOW_ORIGIN", "")
	if isAllowedOrigin("https://a.example.com") {
		t.Fatalf("empty allow list should deny")
	}
}

func TestSanitizeBackupPath(t *testing.T) {
	oldBackup := backupDirName
	t.Cleanup(func() {
		backupDirName = oldBackup
	})

	backupDirName = t.TempDir()
	good := backupDirName + string(os.PathSeparator) + "bedboard_20260529.db"
	clean, err := sanitizeBackupPath(good)
	if err != nil {
		t.Fatalf("expected good backup path: %v", err)
	}
	if !strings.HasPrefix(clean, backupDirName) {
		t.Fatalf("expected sanitized path inside backup dir")
	}

	bad := backupDirName + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "escape.db"
	if _, err := sanitizeBackupPath(bad); err == nil {
		t.Fatalf("expected traversal path rejection")
	}
}
