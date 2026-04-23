#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

BACKEND_PORT="${BACKEND_PORT:-38080}"
FRONTEND_PORT="${FRONTEND_PORT:-35173}"
MOCK_PORT="${MOCK_PORT:-38081}"

BACKEND_LOG="${ROOT_DIR}/.sisyphus/evidence/test-backend.log"
FRONTEND_LOG="${ROOT_DIR}/.sisyphus/evidence/test-frontend.log"
MOCK_LOG="${ROOT_DIR}/.sisyphus/evidence/test-mock-llm.log"

mkdir -p "${ROOT_DIR}/.sisyphus/evidence"

kill_port() {
  local port="$1"
  local pids
  pids="$(lsof -ti tcp:"${port}" || true)"
  if [[ -n "${pids}" ]]; then
    echo "[test-stack] killing process on port ${port}: ${pids}"
    kill ${pids} || true
    sleep 0.3
  fi
}

kill_port "${BACKEND_PORT}"
kill_port "${FRONTEND_PORT}"
kill_port "${MOCK_PORT}"

cd "${ROOT_DIR}"

echo "[test-stack] starting mock-llm on :${MOCK_PORT}"
go run ./cmd/mock-llm -port "${MOCK_PORT}" -storage ./storage/images -seed 1 >"${MOCK_LOG}" 2>&1 &
MOCK_PID=$!

echo "[test-stack] starting backend on :${BACKEND_PORT}"
SERVER_PORT="${BACKEND_PORT}" \
PROXY_BASE_URL="http://127.0.0.1:${MOCK_PORT}" \
PROXY_AUTH_HEADER_VALUE="mock" \
STORAGE_ROOT="./storage-e2e" \
DB_PATH="./storage-e2e/banana-lab.db" \
go run ./cmd/server >"${BACKEND_LOG}" 2>&1 &
BACKEND_PID=$!

echo "[test-stack] starting frontend on :${FRONTEND_PORT}"
VITE_BACKEND_ORIGIN="http://127.0.0.1:${BACKEND_PORT}" \
npm run dev --prefix web -- --host 127.0.0.1 --port "${FRONTEND_PORT}" >"${FRONTEND_LOG}" 2>&1 &
FRONTEND_PID=$!

cleanup() {
  echo "[test-stack] stopping..."
  kill "${FRONTEND_PID}" "${BACKEND_PID}" "${MOCK_PID}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "[test-stack] ready"
echo "  frontend: http://127.0.0.1:${FRONTEND_PORT}"
echo "  backend : http://127.0.0.1:${BACKEND_PORT}"
echo "  mock    : http://127.0.0.1:${MOCK_PORT}"
echo "[test-stack] logs:"
echo "  ${FRONTEND_LOG}"
echo "  ${BACKEND_LOG}"
echo "  ${MOCK_LOG}"

wait
