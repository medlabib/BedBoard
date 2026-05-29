# Contributing to BedBoard

## Prerequisites

- Go 1.23+
- Node.js 18+
- npm
- bash (Linux/macOS/WSL)

## Setup

```bash
git clone https://github.com/medlabib/BedBoard.git
cd BedBoard
npm --prefix frontend ci
npm --prefix frontend run build
bash scripts/prepare-backend-assets.sh
```

## Run locally

```bash
go run ./backend
```

Open http://localhost:8080.

## Test

Backend:

```bash
go test ./backend/...
```

Frontend:

```bash
npm --prefix frontend run test
```

Coverage:

```bash
go test ./backend/... -coverprofile=coverage.out -covermode=atomic
go tool cover -func=coverage.out | awk '/^total:/{print $NF}'
npm --prefix frontend run test:coverage
```

## Code style

- Keep changes scoped and minimal.
- Use `gofmt` for Go files.
- Keep frontend code lint/test friendly.
- Add/adjust tests when behavior changes.

## Pull Request process

1. Create a feature branch from `main`.
2. Ensure backend and frontend tests pass.
3. Write a clear PR description:
   - Problem
   - Solution
   - Validation steps
4. Link related issues when applicable.

## Security

- Do not commit secrets, tokens, private keys, or real patient data.
- Use placeholders and anonymized fixtures in tests and documentation.
- If you find a vulnerability, open a private security report instead of a public issue.
