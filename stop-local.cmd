@echo off
rem Stops the Recon Tool server started by start-local.cmd. Your data in build\ui.db is kept.
taskkill /im recon-server.exe >nul 2>&1 || taskkill /f /im recon-server.exe >nul 2>&1
if errorlevel 1 (echo The Recon Tool was not running.) else (echo The Recon Tool has stopped.)
ping -n 4 127.0.0.1 >nul
