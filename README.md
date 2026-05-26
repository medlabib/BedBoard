# BedBoard

<p align="center">
  <img src="logo.png" alt="BedBoard" width="120" />
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.23-7fa7d4" />
  <img alt="React" src="https://img.shields.io/badge/React-Vite-7ab893" />
  <img alt="SQLite" src="https://img.shields.io/badge/SQLite-GORM-a58ac9" />
  <img alt="Security" src="https://img.shields.io/badge/Security-Hardened-d97a70" />
</p>

BedBoard is a local hospital bed and patient management system with real-time updates, authentication, role-based permissions, and signed release artifacts.

## Features

- Real-time dashboard using Server-Sent Events (`/api/stream`)
- SQLite persistence with GORM
- Authentication with session cookies and role checks (admin/user)
- Bed management, patient assignment, consultation/archive lifecycle
- Dedicated full-screen patient board view
- Stats tab (consultations, archived patients, average consultation duration)
- Automated release pipeline with signed Windows and Linux artifacts

## Security Pass Summary

Recent hardening includes:

- Strict cookie settings (`HttpOnly`, `SameSite=Strict`, TLS-aware `Secure`)
- Request body size limits on JSON endpoints
- Username/password input bounds and password policy for created users
- CORS origin validation (same-origin by default, optional override via `CORS_ALLOW_ORIGIN`)
- Security headers:
  - `Content-Security-Policy`
  - `X-Content-Type-Options`
  - `X-Frame-Options`
  - `Referrer-Policy`
  - `Permissions-Policy`

## Local Development

Install dependencies and build frontend:

```bash
npm --prefix frontend ci
npm --prefix frontend run build
```

Run backend:

```bash
go run .
```

Open:

- http://localhost:8080

## Default Access

- Admin username: `admin`
- Admin password: `admin123`

## Build Artifacts

Build Windows and Linux binaries manually:

```bash
mkdir -p release
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o release/BedBoard_windows_amd64.exe .
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o release/BedBoard_linux_amd64 .
tar -czf release/BedBoard_linux_amd64.tar.gz -C release BedBoard_linux_amd64
zip -j release/BedBoard_windows_amd64.zip release/BedBoard_windows_amd64.exe
```

## Signed Releases

Workflow:

- `.github/workflows/release-signed.yml`
- Triggered manually with a tag input
- Produces:
  - `BedBoard_windows_amd64.exe`
  - `BedBoard_windows_amd64.zip`
  - `BedBoard_linux_amd64`
  - `BedBoard_linux_amd64.tar.gz`
  - `checksums.txt`
- Signs artifacts using Sigstore Cosign keyless signing and uploads:
  - `.sig` signatures
  - `.pem` signing certificates

### Verify signatures

```bash
cosign verify-blob \
  --signature BedBoard_windows_amd64.exe.sig \
  --certificate BedBoard_windows_amd64.exe.pem \
  BedBoard_windows_amd64.exe

cosign verify-blob \
  --signature BedBoard_linux_amd64.tar.gz.sig \
  --certificate BedBoard_linux_amd64.tar.gz.pem \
  BedBoard_linux_amd64.tar.gz
```

## Optional EV/OV Windows Certificate Signing

If you have a `.pfx` code-signing certificate, the workflow can also apply Authenticode signing before publishing.

Script:

- `scripts/sign-windows.sh`

Secrets:

- `WINDOWS_CERT_BASE64`
- `WINDOWS_CERT_PASSWORD`
- Optional:
  - `TIMESTAMP_URL`
  - `SIGNING_SUBJECT`

## API Overview

- `POST /api/auth`
- `POST /api/logout`
- `GET /api/me`
- `GET /api/stream`
- `POST /api/status`
- `POST /api/config-bed`
- `POST /api/beds`
- `POST|DELETE /api/beds/delete`
- `POST /api/patients`
- `POST /api/patients/archive`
- `GET|POST /api/users` (admin)

## Project Structure

- `main.go`: backend API + DB + SSE + embedded frontend serving
- `frontend/`: React + Vite app
- `scripts/`: helper scripts (including optional signing)
- `.github/workflows/`: CI/release automation
- `release/`: generated release artifacts

## Notes

- This app is designed for local/private deployment environments.
- For internet-facing deployment, place behind HTTPS reverse proxy and set `CORS_ALLOW_ORIGIN` explicitly.
