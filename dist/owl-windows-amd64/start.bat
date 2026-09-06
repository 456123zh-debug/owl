@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion
title owl start
color 0A

cd /d "%~dp0"

echo =============================== owl start ===============================
echo [1/3] checking files...

if not exist "owl.exe" (
  echo [FAIL] missing executable: owl.exe
  pause
  exit /b 1
)
if not exist "MediaServer\MediaServer.exe" (
  echo [FAIL] missing MediaServer binary: MediaServer\MediaServer.exe
  pause
  exit /b 1
)

echo [2/3] starting owl.exe...
echo [INFO] logs will appear in this window
echo [INFO] stop with stop.bat
echo.

set "OWL_ONE_CLICK=1"
"owl.exe"
set "CODE=%ERRORLEVEL%"

echo.
if not "%CODE%"=="0" (
  echo [FAIL] owl.exe exited with code %CODE%
  if exist "run\owl.out" (
    echo --- run\owl.out tail ---
    powershell -NoProfile -Command "Get-Content -Path 'run\owl.out' -Tail 50"
  )
  pause
  exit /b %CODE%
)

echo [OK] owl.exe exited normally
pause
exit /b 0
