#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
web_dir="${CCNVR_WEB_DIR:-${root_dir}/../ccnvr_web}"
template_dir="${root_dir}/dist/owl-windows-amd64"
release_dir="${root_dir}/release"
package_name="owl-windows-amd64"
stage_dir="${release_dir}/${package_name}"

for command in go zip; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    echo "missing required command: ${command}" >&2
    exit 1
  fi
done

if [[ ! -d "${web_dir}" ]]; then
  echo "frontend directory not found: ${web_dir}" >&2
  exit 1
fi
if [[ ! -f "${template_dir}/MediaServer/MediaServer.exe" ]]; then
  echo "MediaServer template not found: ${template_dir}/MediaServer" >&2
  exit 1
fi

echo "[1/4] building frontend"
if command -v pnpm >/dev/null 2>&1; then
  (cd "${web_dir}" && pnpm build)
elif command -v npm >/dev/null 2>&1; then
  (cd "${web_dir}" && npm run build)
else
  echo "pnpm or npm is required to build the frontend" >&2
  exit 1
fi

echo "[2/4] building Windows backend"
mkdir -p "${release_dir}"
(cd "${root_dir}" && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "${release_dir}/owl.exe" ./main.go)

echo "[3/4] assembling clean package"
rm -rf "${stage_dir}"
mkdir -p "${stage_dir}/configs" "${stage_dir}/MediaServer"
cp "${release_dir}/owl.exe" "${stage_dir}/owl.exe"
cp "${template_dir}/start.bat" "${template_dir}/stop.bat" "${template_dir}/README.txt" "${stage_dir}/"
cp "${template_dir}/configs/config.toml" "${stage_dir}/configs/config.toml"
cp -R "${web_dir}/dist" "${stage_dir}/www"

find "${template_dir}/MediaServer" -maxdepth 1 -type f -exec cp {} "${stage_dir}/MediaServer/" \;
rm -rf "${stage_dir}/MediaServer/fflogs" "${stage_dir}/run"

echo "[4/4] creating zip"
rm -f "${release_dir}/${package_name}.zip"
(cd "${release_dir}" && zip -qr "${package_name}.zip" "${package_name}")

echo "package ready: ${release_dir}/${package_name}.zip"
