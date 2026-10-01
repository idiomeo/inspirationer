@echo off
rem ---------------------------------------------------------------------------
rem Inspirationer launcher (Windows)
rem
rem NOTE: this file is intentionally ASCII-only. cmd.exe reads batch files using
rem the current console code page, so non-ASCII text here can be misparsed and
rem produce bogus "not recognized as an internal or external command" errors.
rem Non-English instructions live in HOW-TO-RUN.txt / README.md instead.
rem ---------------------------------------------------------------------------
cd /d "%~dp0"

if not exist inspirationer.exe (
  echo [1/2] inspirationer.exe not found - trying to build it with Go...
  where go >nul 2>nul
  if errorlevel 1 (
    echo     Go 1.20+ was not found. Install it from https://go.dev/dl/ ,
    echo     or use the prebuilt inspirationer.exe from the release archive.
    pause
    exit /b 1
  )
  go build -trimpath -ldflags "-s -w -H=windowsgui" -o inspirationer.exe .
  if errorlevel 1 (
    echo     Build failed - see the errors above.
    pause
    exit /b 1
  )
)

echo [2/2] Starting Inspirationer...
echo     URL:  http://127.0.0.1:8420  (your browser opens automatically)
echo     Data: %~dp0data
echo     Tray: look for the light-bulb icon in the notification area
echo           left-click to open the UI, right-click for the menu / quit
echo.
start "" "%~dp0inspirationer.exe" -open=true
exit /b 0
