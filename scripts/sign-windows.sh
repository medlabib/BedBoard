#!/usr/bin/env bash
set -euo pipefail

# Optional signer for Windows executable.
# Requires secrets exported as environment variables:
# - WINDOWS_CERT_BASE64: base64-encoded .pfx
# - WINDOWS_CERT_PASSWORD: password for .pfx
# Optional:
# - TIMESTAMP_URL: RFC3161 timestamp URL
# - SIGNING_SUBJECT: expected cert subject (informational)

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <path-to-exe>"
  exit 1
fi

EXE_PATH="$1"
if [[ ! -f "$EXE_PATH" ]]; then
  echo "Executable not found: $EXE_PATH"
  exit 1
fi

REQUIRE_SIGNING="${REQUIRE_WINDOWS_AUTHENTICODE:-false}"

if [[ -z "${WINDOWS_CERT_BASE64:-}" || -z "${WINDOWS_CERT_PASSWORD:-}" ]]; then
  if [[ "$REQUIRE_SIGNING" == "true" ]]; then
    echo "Signing secrets are required but not set."
    exit 1
  fi
  echo "Signing secrets are not set. Skipping signing."
  exit 0
fi

if ! command -v osslsigncode >/dev/null 2>&1; then
  echo "osslsigncode not found. Install it before signing."
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

PFX_FILE="$TMP_DIR/cert.pfx"
SIGNED_FILE="$TMP_DIR/signed.exe"
TS_URL="${TIMESTAMP_URL:-http://timestamp.digicert.com}"

printf "%s" "$WINDOWS_CERT_BASE64" | base64 -d > "$PFX_FILE"

osslsigncode sign \
  -pkcs12 "$PFX_FILE" \
  -pass "$WINDOWS_CERT_PASSWORD" \
  -n "BedBoard" \
  -i "https://github.com/medlabib/BedBoard" \
  -h sha256 \
  -t "$TS_URL" \
  -in "$EXE_PATH" \
  -out "$SIGNED_FILE"

mv "$SIGNED_FILE" "$EXE_PATH"
echo "Signed executable: $EXE_PATH"
if [[ -n "${SIGNING_SUBJECT:-}" ]]; then
  echo "Expected subject hint: $SIGNING_SUBJECT"
fi

# Verify Authenticode presence and optionally assert expected subject.
# For self-signed certs, chain trust validation may fail; we still accept if a
# signature is present and signer subject matches the expected hint.
VERIFY_OUT="$(osslsigncode verify "$EXE_PATH" 2>&1 || true)"
if printf "%s" "$VERIFY_OUT" | grep -qi "No signature found"; then
  echo "Authenticode verification failed: no signature found."
  printf "%s\n" "$VERIFY_OUT"
  exit 1
fi
if ! printf "%s" "$VERIFY_OUT" | grep -qi "Signature Index:"; then
  echo "Authenticode verification failed: signature metadata missing."
  printf "%s\n" "$VERIFY_OUT"
  exit 1
fi
if [[ -n "${SIGNING_SUBJECT:-}" ]] && ! printf "%s" "$VERIFY_OUT" | grep -qi "${SIGNING_SUBJECT}"; then
  echo "Authenticode signer subject did not match SIGNING_SUBJECT hint."
  printf "%s\n" "$VERIFY_OUT"
  exit 1
fi
echo "Authenticode signature detected and signer subject validated."
