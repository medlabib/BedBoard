# BedBoard

BedBoard is a Go web app for tracking hospital beds and patients in real time.

The frontend is built with React and Vite, then served by the Go backend from `frontend/dist`.

## Run locally

Install the frontend dependencies once:

```bash
cd frontend && npm install
```

Build the React app:

```bash
cd frontend && npm run build
```

```bash
go run .
```

Then open [http://localhost:8080](http://localhost:8080).

## What it does

- Serves the UI from the embedded [index.html](index.html).
- Persists beds, patients, admin users, and sessions in SQLite through GORM.
- Streams live updates to the browser with Server-Sent Events on `/api/stream`.
- Provides role-based access:
	- authenticated users can change bed status and assign patients;
	- admins can create and delete beds, manage users, and edit bed metadata.

## Useful defaults

- Admin username: `admin`
- Admin password: `admin123`
- Default beds: 5 beds, with the 5th set to `thoracique`

## Windows build

```bash
set GOOS=windows && set GOARCH=amd64 && go build -ldflags="-s -w" -o bedboard.exe .
```