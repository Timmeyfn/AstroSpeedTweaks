@echo off
setlocal
cd /d "%~dp0"
title AstroSpeed builder
echo.
echo  AstroSpeed builder
echo  -------------------
echo.

set "PY="
python --version >nul 2>&1 && set "PY=python"
if not defined PY py --version >nul 2>&1 && set "PY=py"
if not defined PY goto nopython
go version >nul 2>&1 || goto nogo
if not exist catalog.go goto wrongfolder

rem one-time fix: use the normal Go download server on this PC
findstr /b /c:"replace golang.org/x/sys" go.mod >nul 2>&1 && (
  echo Setting up Go modules, first time only...
  go mod edit -dropreplace golang.org/x/sys || goto fail
  go mod tidy || goto fail
)

rem find or install go-winres, which adds the icon and the admin prompt
set "WINRES=go-winres"
where go-winres >nul 2>&1 && goto havewinres
for /f "delims=" %%i in ('go env GOPATH') do set "WINRES=%%i\bin\go-winres.exe"
if exist "%WINRES%" goto havewinres
echo Installing go-winres, first time only...
go install github.com/tc-hib/go-winres@latest || goto fail
:havewinres

set GOOS=windows
set GOARCH=amd64

echo [1/5] Reading your .bat files from the custom folder...
%PY% tools\bat2tweak.py custom || goto fail

echo [2/5] Checking every tweak follows the rules...
go test . > "%TEMP%\astrospeed-check.txt" 2>&1 || goto testfail

echo [3/5] Building the interface...
%PY% build_ui.py >nul || goto fail

echo [4/5] Adding the icon and admin prompt...
"%WINRES%" make --in winres\winres.json --out rsrc || goto fail
if exist rsrc_windows_386.syso del rsrc_windows_386.syso

echo [5/5] Building AstroSpeed Tweaks.exe...
go build -trimpath -ldflags "-H windowsgui -s -w" -o "AstroSpeed Tweaks.exe" . || goto fail

echo.
echo  Done! Your new app is "AstroSpeed Tweaks.exe" in this folder.
echo  Your .bat tweaks are in the app under "Your tweaks".
echo.
explorer /select,"%~dp0AstroSpeed Tweaks.exe"
pause
exit /b 0

:testfail
type "%TEMP%\astrospeed-check.txt"
echo.
echo  A tweak broke one of AstroSpeed's rules - see the lines above.
echo  Lines that start with bat- are from your .bat files.
echo  Remove that line from your .bat file, then run build.bat again.
goto end

:nopython
echo  Python is not installed. Download it from https://www.python.org/downloads
echo  During setup, tick "Add python.exe to PATH". Then run build.bat again.
goto end

:nogo
echo  Go is not installed. Download it from https://go.dev/dl
echo  Install it, close this window, then run build.bat again.
goto end

:wrongfolder
echo  build.bat must be in the same folder as catalog.go - the unzipped source folder.
goto end

:fail
echo.
echo  The build stopped because of the error above.

:end
echo.
pause
exit /b 1
