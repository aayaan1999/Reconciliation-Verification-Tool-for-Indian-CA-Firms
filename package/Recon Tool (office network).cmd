@echo off
rem Starts the Recon Tool so that other computers in the SAME office network can use it.
rem Run this on ONE office computer. It keeps running after you close the window, so
rem colleagues can keep working; stop it with "Stop Recon Tool.cmd". The data stays in
rem recon.db in this folder, on this computer only.
rem Colleagues open the address shown below in a browser. Everyone signs in with their own
rem login (the Partner adds users in the tool).
setlocal
cd /d "%~dp0"
set "STATE=%LOCALAPPDATA%\ReconTool"
if not exist "%STATE%" mkdir "%STATE%"

rem This computer's office-network address.
set "IP="
for /f "delims=" %%a in ('powershell -NoProfile -ExecutionPolicy Bypass -Command "$r = Get-NetRoute -DestinationPrefix '0.0.0.0/0' | Sort-Object { $_.RouteMetric + $_.InterfaceMetric } | Select-Object -First 1; (Get-NetIPAddress -AddressFamily IPv4 -InterfaceIndex $r.ifIndex | Where-Object { $_.IPAddress -notlike '169.254.*' } | Select-Object -First 1).IPAddress"') do set "IP=%%a"
if not defined IP (
  echo No office-network address was found. Connect this computer to the office network and try again.
  pause
  exit /b 1
)
set "URL=http://%IP%:8080"

curl.exe -s -o nul "%URL%"
if errorlevel 1 (
  rem A copy running on this computer only (127.0.0.1) cannot be shared: stop it first.
  curl.exe -s -o nul "http://127.0.0.1:8080"
  if not errorlevel 1 (
    echo The tool is already running for this computer only. Close its window, then run this again.
    pause
    exit /b 1
  )
  powershell -NoProfile -ExecutionPolicy Bypass -Command ^
    "$p = Start-Process -FilePath '.\recon-server.exe' -ArgumentList '--listen','%IP%:8080','--host','%COMPUTERNAME%','--db','recon.db' -WindowStyle Hidden -PassThru; $p.Id | Set-Content -Path '%STATE%\server.pid'"
  for /l %%i in (1,1,30) do (
    curl.exe -s -o nul "%URL%" && goto ready
    ping -n 2 127.0.0.1 >nul
  )
  echo The tool did not start. Run recon-server.exe in a Command Prompt in this folder to see the error.
  pause
  exit /b 1
)
:ready
echo.
echo   The Recon Tool is running for the office network.
echo.
echo   Colleagues open this address in a browser:   %URL%
echo   (or  http://%COMPUTERNAME%:8080 )
echo.
echo   If a colleague cannot connect, Windows Firewall on this computer may need to allow
echo   port 8080: ask whoever looks after the office computers.
echo   Only computers inside your office network can reach it. Do not open it to the internet.
echo.
echo   You can close this window; the tool keeps running.
echo   To stop it, double-click "Stop Recon Tool.cmd".
echo.
start "" "%URL%"
pause
exit /b 0
