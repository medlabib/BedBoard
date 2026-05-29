# BedBoard

<p align="center">
  <img src="logo.svg" alt="BedBoard" width="120" />
</p>

<p align="center">
  <img alt="Local Deployment" src="https://img.shields.io/badge/Local%20Deployment-Ready-B5C7A4" />
  <img alt="Realtime" src="https://img.shields.io/badge/Realtime-Beds%20and%20Patients-A6B8C7" />
  <img alt="Security" src="https://img.shields.io/badge/Security-In%20App%20Controls-E2DACD" />
</p>

BedBoard is a local-first emergency board for bed occupancy and patient flow, with role-based access, realtime sync, and admin-managed security/UI settings.

## Highlights

- Login-first UX: unauthenticated users only see the sign-in page (no sign-up).
- Realtime synchronization via SSE (`/api/stream`).
- Bed status management: free, occupied, cleaning, alert.
- Patient lifecycle: register, assign, consult/archive.
- Roles: `admin`, `user`, `triage`, `reception`, `dechocage`.
- Admin operations: users, backups, restore, security health, integrations.
- White-label + locale controlled by admin (French, English, Arabic).
- Gotify integration with secure token storage and built-in test action.

## Tech Stack

- Backend: Go, GORM, SQLite
- Frontend: React + Vite
- Transport: REST + Server-Sent Events

## Quick Start

```bash
npm --prefix frontend ci
npm --prefix frontend run build
go run .
```

Default URL: `http://localhost:8080`

## First Login

On a fresh database:

- Username: `admin`
- Password: `ChangeMe!123`

Change it immediately.

## Admin Settings Structure

Settings UI is split into sections for better UX:

- Parameters
  - App name, app logo, interface language
  - User creation and password reset
- Security
  - Bootstrap admin credentials
  - Cookie/HSTS/proxy controls
  - Security health checks
- Integrations
  - Gotify URL/token/priority
  - Send test notification button
- Operations
  - One-click backup and restore
  - Audit logs

## Branding and Localization

Branding and locale are persisted in app settings and applied globally:

- Login page
- Main dashboard
- Reception page
- Patient display page

Supported locales:

- `fr`
- `en`
- `ar`

Endpoints:

- `GET /api/public/ui-config`
- `GET /api/admin/ui/config`
- `POST /api/admin/ui/config`

## Security Configuration (In-App)

Admin-configurable security keys:

- `security.admin_init_username`
- `security.admin_init_password`
- `security.force_secure_cookie`
- `security.trust_proxy_headers`
- `security.enable_hsts`
- `security.hsts_max_age`
- `security.hsts_include_subdomains`
- `security.hsts_preload`
- `security.gotify_token_enc_key`

Notes:

- Values are persisted in SQLite app settings.
- Environment variables are fallback sources when app settings are unset.
- For production, configure in-app values before exposure.

## Gotify Integration

Admin endpoints:

- `GET /api/admin/integrations/gotify`
- `POST /api/admin/integrations/gotify`
- `POST /api/admin/integrations/gotify/test`

Behavior:

- URL must be valid `http` or `https`.
- Enabling requires a usable token.
- Token is stored encrypted at rest when encryption key is configured.
- Test endpoint returns explicit errors for bad URL/token/response.

## Security Health Endpoint

- `GET /api/admin/security/health`

Returns:

- Global status (`pass`, `warn`, `fail`)
- Per-check details and recommendations

## Local Build and Release

Validation:

```bash
npm --prefix frontend run build
go build ./...
```

Local release artifacts:

```bash
set +u
npm --prefix frontend run build
mkdir -p release
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags='-s -w' -o release/BedBoard_windows_amd64.exe .
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o release/BedBoard_linux_amd64 .
rm -f release/BedBoard_windows_amd64.zip release/BedBoard_linux_amd64.tar.gz release/checksums.txt
zip -j release/BedBoard_windows_amd64.zip release/BedBoard_windows_amd64.exe
tar -czf release/BedBoard_linux_amd64.tar.gz -C release BedBoard_linux_amd64
sha256sum release/BedBoard_windows_amd64.exe release/BedBoard_windows_amd64.zip release/BedBoard_linux_amd64 release/BedBoard_linux_amd64.tar.gz > release/checksums.txt
```

Generated files:

- `release/BedBoard_windows_amd64.exe`
- `release/BedBoard_windows_amd64.zip`
- `release/BedBoard_linux_amd64`
- `release/BedBoard_linux_amd64.tar.gz`
- `release/checksums.txt`

## GitHub Release

The repository includes a tag-driven GitHub Actions workflow in `.github/workflows/release-signed.yml`.

How it works:

- Push a tag matching `v*`.
- The workflow builds the frontend and backend.
- A security health gate runs before packaging.
- Release artifacts are signed with Sigstore Cosign keyless signing.
- GitHub Release assets are published automatically with checksums and signature material.

Typical commands:

```bash
git add .
git commit -m "release: prepare next version"
git push origin main
git tag vX.Y.Z
git push origin vX.Y.Z
```

Published release assets include:

- binaries and archives
- `checksums.txt`
- `.sig` signatures
- `.pem` certificates emitted by Cosign

## Operational Checklist

1. Login as admin and rotate admin password.
2. Configure branding and locale.
3. Configure security settings and check health endpoint.
4. Configure Gotify and run test notification.
5. Create role accounts and validate permissions.
6. Validate backups and restore on a test copy.
7. Run local release and verify checksums.

## Notes

- Project is local-first; protect exposure with network controls.
- Prefer HTTPS reverse proxy in production.
- Rotate bootstrap/admin credentials and encryption keys regularly.
