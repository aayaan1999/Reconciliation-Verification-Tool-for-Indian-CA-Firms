@echo off
rem Opens the Recon Tool in its own window, like an ordinary program.
rem There is no server window and no browser tab. Closing the window stops the tool.
rem Everything stays on this computer (127.0.0.1); nobody else on the network can reach it.
rem To let other computers in the office use it, use "Recon Tool (office network).cmd" instead.
setlocal
cd /d "%~dp0"
set "URL=http://127.0.0.1:8080"
set "STATE=%LOCALAPPDATA%\ReconTool"
if not exist "%STATE%" mkdir "%STATE%"

rem Already running (for example the office-network copy)? Just open the window.
set "STARTED="
curl.exe -s -o nul "%URL%"
if errorlevel 1 (
  powershell -NoProfile -ExecutionPolicy Bypass -Command ^
    "$p = Start-Process -FilePath '.\recon-server.exe' -ArgumentList '--listen','127.0.0.1:8080','--db','recon.db' -WindowStyle Hidden -PassThru; $p.Id | Set-Content -Path '%STATE%\server.pid'"
  set "STARTED=1"
  for /l %%i in (1,1,30) do (
    curl.exe -s -o nul "%URL%" && goto ready
    ping -n 2 127.0.0.1 >nul
  )
  echo The tool did not start. Run recon-server.exe in a Command Prompt in this folder to see the error.
  pause
  goto stop
)
:ready

rem Find a browser that can show the tool as an app window (Edge comes with Windows).
set "BROWSER="
for %%b in (
  "%ProgramFiles(x86)%\Microsoft\Edge\Application\msedge.exe"
  "%ProgramFiles%\Microsoft\Edge\Application\msedge.exe"
  "%ProgramFiles%\Google\Chrome\Application\chrome.exe"
  "%ProgramFiles(x86)%\Google\Chrome\Application\chrome.exe"
  "%LOCALAPPDATA%\Google\Chrome\Application\chrome.exe"
) do if not defined BROWSER if exist %%b set "BROWSER=%%~b"

if not defined BROWSER (
  echo No Edge or Chrome found, so the tool opens in your browser instead.
  echo It keeps running; double-click "Stop Recon Tool.cmd" when you are done.
  start "" "%URL%"
  exit /b 0
)

rem A separate profile makes this its own window that this script can wait for.
start /wait "" "%BROWSER%" --app=%URL% --user-data-dir="%STATE%\window" --no-first-run --no-default-browser-check --window-size=1366,850

:stop
if defined STARTED if exist "%STATE%\server.pid" (
  for /f %%p in (%STATE%\server.pid) do taskkill /fi "pid eq %%p" /fi "imagename eq recon-server.exe" /f >nul 2>&1
  del "%STATE%\server.pid" >nul 2>&1
)
exit /b 0
