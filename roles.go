package main

import "strings"

func normalizeRole(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case roleAdmin:
		return roleAdmin
	case roleDechocage:
		return roleDechocage
	case roleReception:
		return roleReception
	case roleTriage:
		return roleTriage
	default:
		return roleUser
	}
}

func roleOf(user AdminUser) string {
	if strings.TrimSpace(user.Role) != "" {
		return normalizeRole(user.Role)
	}
	if user.Username == defaultUsername {
		return roleAdmin
	}
	return roleUser
}

func isAdminLike(user AdminUser) bool {
	role := roleOf(user)
	return role == roleAdmin || role == roleDechocage
}

func canManageBeds(user AdminUser) bool {
	role := roleOf(user)
	return role == roleAdmin || role == roleUser || role == roleDechocage
}

func canManagePatients(user AdminUser) bool {
	role := roleOf(user)
	return role == roleAdmin || role == roleUser || role == roleTriage || role == roleDechocage
}

func canArchivePatients(user AdminUser) bool {
	role := roleOf(user)
	return role == roleAdmin || role == roleUser || role == roleDechocage
}
