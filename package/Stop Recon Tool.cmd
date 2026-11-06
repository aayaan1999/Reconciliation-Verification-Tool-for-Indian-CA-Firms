@echo off
rem Stops the Recon Tool that was started from this folder (either way of starting it).
setlocal
set "PIDFILE=%LOCALAPPDATA%\ReconTool\server.pid"
if exist "%PIDFILE%" (
  for /f %%p in (%PIDFILE%) do taskkill /fi "pid eq %%p" /fi "imagename eq recon-server.exe" /f >nul 2>&1
  del "%PIDFILE%" >nul 2>&1
  echo Recon Tool stopped.
) else (
  echo The Recon Tool is not running.
)
timeout /t 2 >nul
