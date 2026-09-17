@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion
title owl uninstall tools
color 0A

cd /d "%~dp0"

if exist "run\owl.pid" (
  set /p PID=<"run\owl.pid"
  if defined PID (
    taskkill /PID !PID! /F >nul 2>&1
  )
  del /f /q "run\owl.pid" >nul 2>&1
)

taskkill /IM owl.exe /F /T >nul 2>&1
taskkill /IM MediaServer.exe /F /T >nul 2>&1

echo [owl] 完成
