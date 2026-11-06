@echo off
rem Makes the package to give to a CA firm: build\ReconTool-release\ and build\ReconTool-for-CA.zip
rem Run this on the development computer after "cd web && npm ci && npm run build".
rem Give the .zip to the firm. Every firm unzips its own copy, and its own empty database
rem (recon.db) and its own first Partner are created on that firm's computer at first start.
rem NEVER run the tool from build\ReconTool-release: running it creates recon.db there. To try
rem the package, unzip a copy somewhere else. This script refuses to continue, and refuses to
rem pack, if any database file is found in the package.
setlocal
cd /d "%~dp0"
set "OUT=build\ReconTool-release"
set "ZIP=build\ReconTool-for-CA.zip"
set "PATH=C:\Program Files\Go\bin;%PATH%"

if not exist "web\dist" (
  echo The screens are not built. Run:  cd web ^&^& npm ci ^&^& npm run build
  exit /b 1
)

rem The release folder is never run, so it holds no data. If it does, stop and let a person look.
if exist "%OUT%\*.db*" (
  echo STOPPED: %OUT% holds a database file ^(recon.db^). It may contain client data.
  echo Move or delete it yourself, then run this again.
  exit /b 1
)
if exist "%OUT%" rmdir /s /q "%OUT%"
mkdir "%OUT%" || exit /b 1

echo Building the programs...
go build -tags ui -o %OUT%/ ./cmd/recon-engine ./cmd/recon-server || exit /b 1

echo Copying rules and content...
robocopy "content" "%OUT%\content" /E /NFL /NDL /NJH /NJS >nul
robocopy "docs\Recon_Tool_YAML_Rule_Store\recon_yaml" "%OUT%\docs\Recon_Tool_YAML_Rule_Store\recon_yaml" /E /NFL /NDL /NJH /NJS >nul
if exist "third_party" robocopy "third_party" "%OUT%\third_party" /E /NFL /NDL /NJH /NJS >nul
copy /y "package\Recon Tool.cmd" "%OUT%\" >nul || exit /b 1
copy /y "package\Recon Tool (office network).cmd" "%OUT%\" >nul || exit /b 1
copy /y "package\Stop Recon Tool.cmd" "%OUT%\" >nul || exit /b 1
copy /y "package\READ ME FIRST.txt" "%OUT%\" >nul || exit /b 1

rem Check: no database, key or private file may be in the package.
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$bad = Get-ChildItem -Recurse -File -Path '%OUT%' | Where-Object { $_.Name -match '\.(db|sqlite|key|pem)$|\.db-(wal|shm|journal)$' };" ^
  "if ($bad) { $bad | ForEach-Object { Write-Host ('NOT ALLOWED IN THE PACKAGE: ' + $_.FullName) }; exit 1 }"
if errorlevel 1 exit /b 1

if exist "%ZIP%" del /q "%ZIP%"
powershell -NoProfile -ExecutionPolicy Bypass -Command "Compress-Archive -Path '%OUT%\*' -DestinationPath '%ZIP%'" || exit /b 1

echo.
echo Done. Give this file to the firm:  %ZIP%
echo Do not run the tool from %OUT%. Unzip a copy elsewhere to try it.
exit /b 0
