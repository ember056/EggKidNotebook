#!/usr/bin/env bash
set -Eeuo pipefail

# Read-only deployment diagnostics for EggKidNotebook / WeKnora.
# This script deliberately avoids printing secrets and never mutates services.

APP_URL="${APP_URL:-http://127.0.0.1:8080}"
FRONTEND_URL="${FRONTEND_URL:-http://127.0.0.1}"
DIAGNOSTICS_URL="${DIAGNOSTICS_URL:-$APP_URL/api/v1/system/admin/diagnostics}"

ok() { printf '\033[32m[OK]\033[0m %s\n' "$*"; }
warn() { printf '\033[33m[WARN]\033[0m %s\n' "$*"; }
fail() { printf '\033[31m[FAIL]\033[0m %s\n' "$*"; }
info() { printf '\033[36m[INFO]\033[0m %s\n' "$*"; }

have() {
  command -v "$1" >/dev/null 2>&1
}

http_code() {
  local url="$1"
  curl -fsS -o /dev/null -w '%{http_code}' --connect-timeout 5 --max-time 15 "$url" 2>/dev/null || true
}

section() {
  printf '\n== %s ==\n' "$1"
}

section "Host"
info "cwd: $(pwd)"
info "time: $(date -Is)"
if have df; then
  df -h . || true
fi
if have free; then
  free -h || true
fi

section "Project files"
if [[ -f ".env" ]]; then
  ok ".env exists"
else
  warn ".env is missing; container defaults may still work, but deployment is harder to audit."
fi
if [[ -f "docker-compose.yml" || -f "compose.yaml" || -f "compose.yml" ]]; then
  ok "docker compose file found"
else
  warn "docker compose file not found in current directory"
fi

section "Docker"
if have docker; then
  ok "docker cli available"
  if docker info >/dev/null 2>&1; then
    ok "docker daemon reachable"
    if docker compose version >/dev/null 2>&1; then
      docker compose ps || true
    else
      warn "docker compose plugin is unavailable"
    fi
  else
    fail "docker daemon is not reachable"
  fi
else
  fail "docker cli is not installed"
fi

section "HTTP"
health_code="$(http_code "$APP_URL/health")"
if [[ "$health_code" == "200" ]]; then
  ok "backend health: $APP_URL/health"
else
  fail "backend health returned '${health_code:-no response}' from $APP_URL/health"
fi

frontend_code="$(http_code "$FRONTEND_URL/")"
if [[ "$frontend_code" =~ ^(200|301|302|304)$ ]]; then
  ok "frontend reachable: $FRONTEND_URL/ ($frontend_code)"
else
  fail "frontend returned '${frontend_code:-no response}' from $FRONTEND_URL/"
fi

section "Platform diagnostics"
if [[ -n "${WEKNORA_PLATFORM_API_KEY:-}" ]]; then
  tenant_header=()
  if [[ -n "${WEKNORA_TENANT_ID:-}" ]]; then
    tenant_header=(-H "X-Tenant-ID: ${WEKNORA_TENANT_ID}")
  fi
  body="$(curl -fsS --connect-timeout 5 --max-time 20 \
    -H "Authorization: Bearer ${WEKNORA_PLATFORM_API_KEY}" \
    "${tenant_header[@]}" \
    "$DIAGNOSTICS_URL" 2>/dev/null || true)"
  if [[ -n "$body" ]]; then
    ok "diagnostics endpoint responded"
    if have jq; then
      printf '%s\n' "$body" | jq '{status, generated_at, checks: [.checks[] | {key, status, message, remediation}]}'
    else
      printf '%s\n' "$body"
    fi
  else
    fail "diagnostics endpoint did not return a readable response"
  fi
else
  warn "skip authenticated diagnostics; set WEKNORA_PLATFORM_API_KEY to call $DIAGNOSTICS_URL"
fi

section "Recent app logs"
if have docker && docker info >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  docker compose logs app --tail=60 2>/dev/null || warn "app logs unavailable"
fi
