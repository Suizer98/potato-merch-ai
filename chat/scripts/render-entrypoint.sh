#!/bin/sh
set -eu

# gRPC stays internal to this container; Render's public PORT serves grpcui HTTP.
export GRPC_ADDR="${GRPC_ADDR:-127.0.0.1:50051}"
UI_PORT="${PORT:-8080}"

echo "starting gRPC on ${GRPC_ADDR} (provider=${LLM_PROVIDER:-mock})"
/server &
SERVER_PID=$!

cleanup() {
  kill "${SERVER_PID}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "waiting for gRPC health"
i=0
until /grpc_health_probe -addr="${GRPC_ADDR}"; do
  i=$((i + 1))
  if [ "${i}" -gt 40 ]; then
    echo "gRPC failed to become healthy"
    exit 1
  fi
  sleep 0.25
done

echo "starting grpcui on 0.0.0.0:${UI_PORT} -> ${GRPC_ADDR}"
exec /grpcui -plaintext -open-browser=false -port "${UI_PORT}" -bind 0.0.0.0 "${GRPC_ADDR}"
