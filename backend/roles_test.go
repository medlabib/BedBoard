package main

import "testing"

func TestRoleNormalizationAndPermissions(t *testing.T) {
	if got := normalizeRole("  ADMIN "); got != roleAdmin {
		t.Fatalf("normalizeRole admin: got %q", got)
	}
	if got := normalizeRole("unknown"); got != roleUser {
		t.Fatalf("normalizeRole fallback user: got %q", got)
	}

	admin := AdminUser{Username: defaultUsername, Role: roleAdmin}
	triage := AdminUser{Username: "t", Role: roleTriage}
	reception := AdminUser{Username: "r", Role: roleReception}

	if !isAdminLike(admin) {
		t.Fatalf("admin should be admin-like")
	}
	if canManageBeds(triage) {
		t.Fatalf("triage should not manage beds")
	}
	if !canManagePatients(triage) {
		t.Fatalf("triage should manage patients")
	}
	if canArchivePatients(reception) {
		t.Fatalf("reception should not archive patients")
	}
}
