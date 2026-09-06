#!/usr/bin/env bash
set -u

RED='\033[31m'
GREEN='\033[32m'
YELLOW='\033[1;33m'
NC='\033[0m'

cd "$(dirname "$0")"

echo -e "${GREEN}============================= owl uninstall ==============================${NC}"
echo

if [ -f "run/owl.pid" ]; then
  PID=$(cat "run/owl.pid" 2>/dev/null || true)
  if [ -n "${PID}" ] && kill -0 "${PID}" 2>/dev/null; then
    kill "${PID}" 2>/dev/null || true
    sleep 1
  fi
  rm -f "run/owl.pid"
fi

pkill -f "$(pwd)/owl" 2>/dev/null || true
pkill -f "$(pwd)/MediaServer" 2>/dev/null || true

if pgrep -f "$(pwd)/owl" >/dev/null 2>&1 || pgrep -f "$(pwd)/MediaServer" >/dev/null 2>&1; then
  echo -e "${YELLOW}[WARN] some processes may still be running${NC}"
  exit 1
fi

echo -e "${GREEN}[OK] uninstall completed${NC}"
