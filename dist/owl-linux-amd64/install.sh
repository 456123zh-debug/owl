#!/usr/bin/env bash
set -u

RED='\033[31m'
GREEN='\033[32m'
YELLOW='\033[1;33m'
NC='\033[0m'

cd "$(dirname "$0")"

echo -e "${GREEN}=============================== owl install ===============================${NC}"
echo

if [ ! -x "./owl" ]; then
  echo -e "${RED}[FAIL] missing executable: ./owl${NC}"
  exit 1
fi
if [ ! -x "./MediaServer/MediaServer" ] && [ ! -x "./MediaServer/MediaServer.exe" ]; then
  echo -e "${RED}[FAIL] missing MediaServer binary${NC}"
  exit 1
fi
if [ ! -f "./www/index.html" ]; then
  echo -e "${RED}[FAIL] missing frontend assets: ./www/index.html${NC}"
  exit 1
fi

if [ -f "run/owl.pid" ]; then
  PID=$(cat "run/owl.pid" 2>/dev/null || true)
  if [ -n "${PID}" ] && kill -0 "${PID}" 2>/dev/null; then
    echo -e "${YELLOW}[WARN] owl is already running (pid ${PID})${NC}"
    exit 1
  fi
fi

./start.sh
status=$?
if [ $status -eq 0 ]; then
  echo -e "${GREEN}[OK] install completed${NC}"
  exit 0
fi

echo -e "${RED}[FAIL] install failed${NC}"
if [ -f "run/owl.out" ]; then
  echo -e "${YELLOW}--- run/owl.out tail ---${NC}"
  tail -n 80 run/owl.out 2>/dev/null || true
fi
exit $status
