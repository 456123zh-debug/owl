#!/usr/bin/env bash
set -u

# owl 视频监控平台 一键停止
cd "$(dirname "$0")"

if [ -f "run/owl.pid" ]; then
  PID=$(cat "run/owl.pid" 2>/dev/null)
  if [ -n "${PID}" ] && kill -0 "${PID}" 2>/dev/null; then
    kill "${PID}" && echo "[owl] 已停止 owl (pid ${PID})"
  fi
  rm -f "run/owl.pid"
fi

# 兜底
pkill -f "$(pwd)/owl" 2>/dev/null
pkill -f "$(pwd)/MediaServer" 2>/dev/null
echo "[owl] 完成"
