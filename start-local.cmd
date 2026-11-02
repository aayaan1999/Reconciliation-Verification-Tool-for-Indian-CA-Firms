@echo off
rem Starts the Recon Tool on this computer only and opens it in the browser.
rem Double-click to use. Close the "Recon Tool server" window (or run stop-local.cmd) to stop it.
rem Everything stays on this machine: the server listens on 127.0.0.1 only.
rem The programs are built the first time, and built again whenever a source file (Go code,
rem go.mod, go.sum or the screens in web\) is newer than they are. Rules, profiles and other
rem content are read when the tool runs, so changing them needs no build.
setlocal
cd /d "%~dp0"
set "URL=http://127.0.0.1:8080"

rem Is the build missing or older than the source?
set "STALE="
if not exist "build\recon-server.exe" set "STALE=1"
if not exist "build\recon-engine.exe" set "STALE=1"
if not defined STALE (
  powershell -NoProfile -ExecutionPolicy Bypass -Command ^
    "$built = @('build\recon-server.exe','build\recon-engine.exe' | ForEach-Object { (Get-Item $_).LastWriteTimeUtc } | Sort-Object)[0];" ^
    "$src = @(Get-ChildItem -Recurse -File -Path cmd,internal -Filter *.go | Where-Object { $_.Name -notlike '*_test.go' });" ^
    "$src += Get-ChildItem -Recurse -File -Path web\src,web\public -ErrorAction SilentlyContinue;" ^
    "$src += Get-Item go.mod,go.sum,web\*.go,web\index.html,web\package.json,web\package-lock.json,web\vite.config.ts,web\tsconfig*.json -ErrorAction SilentlyContinue | Where-Object { $_.Name -notlike '*_test.go' };" ^
    "$newer = $src | Where-Object { $_.LastWriteTimeUtc -gt $built } | Select-Object -First 1;" ^
    "if ($newer) { Write-Host ('Source changed since the last build: ' + $newer.FullName.Substring((Get-Location).Path.Length + 1)); exit 1 } else { exit 0 }"
  if errorlevel 1 set "STALE=1"
)

rem Already running? Just open the browser; a running copy cannot be rebuilt while it runs.
curl.exe -s -o nul "%URL%"
if not errorlevel 1 (
  if defined STALE (
    echo The running copy is older than the source. Run stop-local.cmd, then start-local.cmd again to use the new code.
    pause
  )
  goto open
)

if not defined STALE goto start
echo Building the Recon Tool (this takes a minute)...
set "PATH=C:\Program Files\Go\bin;%PATH%"
pushd web
if not exist "node_modules" call npm ci || goto failed
call npm run build || goto failed
popd
go build -tags ui -o build\ ./cmd/recon-engine ./cmd/recon-server || goto failed

:start
echo Starting the Recon Tool...
start "Recon Tool server" "build\recon-server.exe" --listen 127.0.0.1:8080 --db build\ui.db
for /l %%i in (1,1,30) do (
  curl.exe -s -o nul "%URL%" && goto open
  ping -n 2 127.0.0.1 >nul
)
echo The server did not start. Check the "Recon Tool server" window for the error.
pause
exit /b 1

:open
start "" "%URL%"
exit /b 0

:failed
echo Build failed. See the messages above.
pause
exit /b 1
