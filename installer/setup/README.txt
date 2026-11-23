AstroSpeed wizard installer (AstroSpeed-Setup.exe)
==================================================

AstroSpeed-Setup.exe is a normal "Install / Finish" wizard. It copies AstroSpeed
into the user's own programs folder, makes a Start Menu shortcut (and optionally a
desktop one), and adds a proper entry under Settings > Apps so it can be uninstalled.
It installs just for the current user, so it needs no administrator password to run
(the app itself still asks for admin when it actually applies a tweak).

This is built from Go, the same as the app - no extra tools needed.

To rebuild it after a new app build:
  1. Build the app (run build.bat in the main folder) so dist\AstroSpeed Tweaks.exe exists.
  2. Run build-installer.bat in this folder.
  3. Share the AstroSpeed-Setup.exe it produces.

(There is also an AstroSpeed.iss in ..\  for people who prefer Inno Setup, but you
don't need it - the Go installer above is the easy path.)
