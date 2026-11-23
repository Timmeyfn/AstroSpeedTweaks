@echo off
rem Example online tweak: each .bat in this folder becomes one switch in AstroSpeed.
rem The first "rem" line with real words becomes the description.
reg add "HKLM\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters" /v DefaultTTL /t REG_DWORD /d 64 /f
netsh int tcp set global timestamps=disabled
