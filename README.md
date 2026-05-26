# BedBoard

<p align="center">
  <img src="logo.svg" alt="BedBoard" width="120" />
</p>

<p align="center">
  <img alt="Local Deployment" src="https://img.shields.io/badge/Local%20Deployment-Ready-B5C7A4" />
  <img alt="Realtime" src="https://img.shields.io/badge/Realtime-Beds%20and%20Patients-A6B8C7" />
  <img alt="Security" src="https://img.shields.io/badge/Security-Hardened%20by%20Env-E2DACD" />
</p>

BedBoard is a local-first emergency-unit board for bed occupancy and patient flow.

## Features

- Realtime updates via server events.
- Bed state management: free, occupied, cleaning, alert.
- Patient lifecycle: registration, assignment, consulted, archived.
- Role-based access: admin, user, triage, reception, dechocage.
- Audit trail for critical bed operations.
- Admin backup and restore for SQLite.
- Security health endpoint for quick posture checks.

## Quick Start

```bash
npm --prefix frontend ci
npm --prefix frontend run build
go run .
```

Default app URL: `http://localhost:8080`

## WARNING: Default Password and Generated Environment

This section provides an **insecure bootstrap** for quick local validation only.

- Default bootstrap username: `admin`
- Default bootstrap password: `ChangeMe!123`
- You must rotate this password immediately after first login.
- Never use this insecure default profile in production.

### Linux/macOS: generate default env quickly

```bash
cat > .env.default.generated <<'EOF'
ADMIN_INIT_USERNAME=admin
ADMIN_INIT_PASSWORD=ChangeMe!123
FORCE_SECURE_COOKIE=true
TRUST_PROXY_HEADERS=true
ENABLE_HSTS=true
HSTS_MAX_AGE=31536000
HSTS_INCLUDE_SUBDOMAINS=true
HSTS_PRELOAD=false
GOTIFY_TOKEN_ENC_KEY=$(openssl rand -base64 32)
EOF

set -a
source .env.default.generated
set +a

go run .
```

### Windows PowerShell: generate default env quickly

```powershell
$bytes = New-Object byte[] 32
[System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
$gotifyKey = [Convert]::ToBase64String($bytes)

@"
ADMIN_INIT_USERNAME=admin
ADMIN_INIT_PASSWORD=ChangeMe!123
FORCE_SECURE_COOKIE=true
TRUST_PROXY_HEADERS=true
ENABLE_HSTS=true
HSTS_MAX_AGE=31536000
HSTS_INCLUDE_SUBDOMAINS=true
HSTS_PRELOAD=false
GOTIFY_TOKEN_ENC_KEY=$gotifyKey
"@ | Set-Content .env.default.generated

Get-Content .env.default.generated | ForEach-Object {
  if ($_ -match '^(.*?)=(.*)$') {
    [Environment]::SetEnvironmentVariable($matches[1], $matches[2], 'Process')
  }
}

go run .
```

## Mandatory Hardening Before Production

1. Replace `ADMIN_INIT_PASSWORD` with a strong unique secret.
2. Run behind HTTPS and keep `FORCE_SECURE_COOKIE=true`.
3. Keep `ENABLE_HSTS=true` only when HTTPS is active end-to-end.
4. Use a real persistent `GOTIFY_TOKEN_ENC_KEY` and keep it secret.
5. Restrict network exposure (private subnet, VPN, or reverse-proxy ACL).

## Security Environment Variables

- `ADMIN_INIT_USERNAME`: bootstrap admin username (default `admin`).
- `ADMIN_INIT_PASSWORD`: bootstrap admin password (required if no admin exists).
- `FORCE_SECURE_COOKIE`: force `Secure` cookie flag.
- `TRUST_PROXY_HEADERS`: trust `X-Forwarded-Proto=https` for secure cookies.
- `ENABLE_HSTS`: enable `Strict-Transport-Security` header.
- `HSTS_MAX_AGE`: HSTS max age in seconds.
- `HSTS_INCLUDE_SUBDOMAINS`: add `includeSubDomains` token to HSTS.
- `HSTS_PRELOAD`: add `preload` token to HSTS.
- `GOTIFY_TOKEN_ENC_KEY`: base64 key used to encrypt Gotify token at rest.

## Security Health Endpoint (Admin)

- `GET /api/admin/security/health`

Returns:

- global status: `pass`, `warn`, or `fail`
- detailed checks and recommendations

Use this endpoint in CI/CD gates and admin verification flows.

## Build and Release

- CI workflow: `.github/workflows/ci.yml`
- Signed release workflow: `.github/workflows/release-signed.yml`
- Release tags format: `v*` (example: `v2.2.0`)

## Local Test Matrix

1. Login with bootstrap account.
2. Create one patient, assign one bed, set alert status.
3. Verify audit logs update.
4. Trigger backup and restore.
5. Validate security health endpoint returns expected controls.

## Support Notes

- For local testing speed, `.env.default.generated` is acceptable.
- For any shared or production environment, remove insecure defaults immediately.
- Keep README warnings visible for operators and reviewers.