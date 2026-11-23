# AstroSpeed Tweaks

A Windows PC optimizer for gamers: per-game tweaks for 12 games, 150+ reversible tweaks, one-click Optimize, a game booster, a system cleaner, startup manager, Fortnite server ping test and a BIOS checklist.

## Online pack (add tweaks without rebuilding)

Every copy reads `astrospeed.json` and `tweaks/*.bat` from the GitHub repository in `update-repo.txt`, checks them against the same safety rules, and adds them to the app. See `github-repo-starter/HOW TO ADD ONLINE TWEAKS.txt`. Code: `pack.go` (format, safety checks, .bat converter) and `pack_windows.go` (download, cache, live reload).

## Running it

Double-click `AstroSpeed Tweaks.exe` and accept the administrator prompt. It needs the Microsoft Edge WebView2 Runtime, which Windows 10 and 11 already include on nearly every PC.

Because the exe isn't code-signed, Windows SmartScreen may say "Windows protected your PC". Click **More info → Run anyway**.

## What it never touches

Mouse sensitivity, pointer speed, Enhance pointer precision, DPI, mouse/keyboard driver buffers, Fortnite's settings file (GameUserSettings.ini), keybinds, Epic launcher settings, resolution/refresh rate, Defender, Firewall, UAC and Windows Update. `catalog_test.go` enforces this.

## Safety

* Every change is backed up to `C:\ProgramData\AstroSpeed\state.json` before it's made, and Undo restores the exact previous value.
* Optimize offers to create a System Restore point first.
* Security trade-offs (CPU mitigations, Memory Integrity, VBS) and appearance changes are manual-only and never applied by Optimize.
* The app launches no other programs at startup and never uses encoded PowerShell commands. If it ever closes unexpectedly, the next launch shows what happened (logs in `C:\ProgramData\AstroSpeed\logs`).
* If BitLocker is on, it is paused for one restart before any boot-setting change, so you won't be asked for a recovery key.

## Building from source

Requires Go 1.24+ and Python 3.

```
python3 build_ui.py                       # inlines ui/src + fonts into ui/dist/index.html
go-winres make --in winres/winres.json    # icon, admin manifest, version info
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H windowsgui -s -w" -o "AstroSpeed Tweaks.exe" .
go test .                                 # catalog safety rules (set SG_PWSH=pwsh to also parse every PowerShell script)
```

`python3 build_ui.py --preview` (after `SG_DUMP=ui/preview/catalog.json go test -run Dump .`) builds a browser preview of the UI with a mock backend.

## Layout

| File | What it does |
|---|---|
| `catalog.go` | Every tweak: title, description, impact, risk, profile and the exact actions |
| `engine.go` | Applies/reverts registry, service, scheduled task, boot (bcdedit) and command actions with backups |
| `custom.go` | Power plan, network adapter, USB power, memory manager, Defender and Discord handlers |
| `tools.go` | Cleaner, startup apps, game booster (standby memory, timer resolution), ping, DNS, restore points |
| `scripts.go` | All PowerShell scripts, kept in one place so they can be syntax-checked |
| `ui/src` | The interface (HTML/CSS/JS) |

Font: Manrope, SIL Open Font License (see `ui/fonts`). Not affiliated with Epic Games.

## Updates

Put your GitHub repository (`name/repo`) in `update-repo.txt` and rebuild. The app then checks that repository's latest release when it opens and offers the download. See `HOW TO RELEASE AN UPDATE.txt`.
