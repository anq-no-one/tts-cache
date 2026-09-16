#!/usr/bin/env bash
set -euo pipefail

PORT="${PORT:-8080}"
INVITE_CODE="${INVITE_CODE:?set INVITE_CODE to one entry of the server INVITE_CODES list}"
APP_TOKEN="${APP_TOKEN:-}"
APP_NAME="${APP_NAME:-my-app}"
VOICE_ID="${VOICE_ID:-test-voice}"

BASE="http://localhost:${PORT}"

if [ -z "${APP_TOKEN}" ]; then
  REGISTER_JSON="$(curl -s -X POST "${BASE}/v1/register" \
    -H 'Content-Type: application/json' \
    -d "{\"invitation_code\": \"${INVITE_CODE}\", \"app_name\": \"${APP_NAME}\"}")"
  echo "${REGISTER_JSON}"
  APP_TOKEN="$(printf '%s' "${REGISTER_JSON}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["app_token"])')"
fi

curl -s -X POST "${BASE}/v1/synthesize" \
  -H "Authorization: Bearer ${APP_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"text\": \"Hello. World.\", \"voice_id\": \"${VOICE_ID}\"}" \
  -D - -o out.mp3

curl -s "${BASE}/metrics"
echo
