#!/usr/bin/env bash
set -u

# owl 视频监控平台 一键启动
cd "$(dirname "$0")"

[ -x "./owl" ] || {
  echo "[owl] 找不到可执行文件 ./owl"
  exit 1
}

LANIP=$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<NF;i++) if($i=="src"){print $(i+1); exit}}')
[ -z "${LANIP}" ] && LANIP=$(hostname -I 2>/dev/null | awk '{print $1}')
[ -z "${LANIP}" ] && LANIP=127.0.0.1

update_config() {
  local file="configs/config.toml"
  [ -f "${file}" ] || return 0
  local media_secret=""
  if [ -f "MediaServer/config.ini" ]; then
    media_secret=$(awk -F= '/^[[:space:]]*secret[[:space:]]*=/ {gsub(/[[:space:]]/,"",$2); print $2; exit}' MediaServer/config.ini)
  fi
  sed -i -E \
    -e "s|^([[:space:]]*Host = )'[^']*'|\1'${LANIP}'|" \
    -e "s|^([[:space:]]*IP = )'[^']*'|\1'${LANIP}'|" \
    -e "s|^([[:space:]]*WebHookIP = )'[^']*'|\1'${LANIP}'|" \
    -e "s|^([[:space:]]*SDPIP = )'[^']*'|\1'${LANIP}'|" \
    "${file}"
  if [ -n "${media_secret}" ]; then
    media_secret=${media_secret//\\/\\\\}
    media_secret=${media_secret//&/\\&}
    sed -i -E \
      -e "s|^([[:space:]]*Secret = )'[^']*'|\1'${media_secret}'|" \
      "${file}"
  fi
}

check_http() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsS --max-time 2 http://127.0.0.1:15123/health >/dev/null 2>&1
    return $?
  fi
  (exec 3<>/dev/tcp/127.0.0.1/15123) >/dev/null 2>&1
}

update_config

mkdir -p configs run
echo "[owl] 访问地址: http://${LANIP}:15123"
echo "[owl] 启动模式: one-click"

export OWL_ONE_CLICK=1
nohup ./owl > run/owl.out 2>&1 &
echo $! > run/owl.pid

for _ in $(seq 1 30); do
  if check_http; then
    echo "================================================"
    echo " owl 视频监控平台已启动"
    echo ""
    echo " 管理界面 : http://${LANIP}:15123"
    echo " 默认账号 : admin / admin"
    echo ""
    echo " 日志目录 : run/"
    echo " 停止服务 : ./stop.sh"
    echo "================================================"
    exit 0
  fi
  if ! kill -0 "$(cat run/owl.pid)" 2>/dev/null; then
    echo "[owl] 启动失败，查看 run/owl.out"
    tail -n 50 run/owl.out 2>/dev/null || true
    exit 1
  fi
  sleep 1
done

echo "[owl] 启动超时，查看 run/owl.out"
tail -n 50 run/owl.out 2>/dev/null || true
exit 1
