@echo off
rem Builds AstroSpeed-Setup.exe (the wizard installer) from the app exe.
rem Run build.bat in the main folder FIRST so dist\AstroSpeed Tweaks.exe exists.
setlocal
cd /d "%~dp0"
copy /y "..\..\dist\AstroSpeed Tweaks.exe" app_payload.bin >nul || (echo Build the app first with build.bat & exit /b 1)
where go-winres >nul 2>&1 && go-winres make --in winres.json --out rsrc
del /q rsrc_windows_386.syso 2>nul
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -trimpath -ldflags "-H windowsgui -s -w" -o "AstroSpeed-Setup.exe" .
echo Done: installer\setup\AstroSpeed-Setup.exe
endlocal
