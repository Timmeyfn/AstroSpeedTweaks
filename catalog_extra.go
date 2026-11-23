package main

// Tweaks added in 1.6.0. Kept in their own file so the original catalog stays easy to read.

var extraTweaks = []Tweak{
	{
		ID: "fth", Cat: "cpu", Group: "Stability", Title: "Stop Windows slowing down games that crashed once",
		Desc:    "Turns off the Fault Tolerant Heap. After an app crashes a few times, Windows quietly runs it with extra memory checks from then on.",
		Benefit: "A game that crashed last week doesn't keep running with slower memory handling forever.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{RegD(HKLM, `SOFTWARE\Microsoft\FTH`, "Enabled", 0)},
	},
	{
		ID: "maintenance", Cat: "services", Group: "Scheduled work", Title: "Turn off Automatic Maintenance",
		Desc:    "Stops Windows waking up its maintenance jobs (disk scans, diagnostics, app updates) when it thinks the PC is idle, which includes long loading screens.",
		Benefit: "No surprise disk and CPU load in the middle of a session.",
		Note:    "Windows Update still works. Run the System cleaner now and then instead.",
		Impact:  2, Risk: "moderate", Profile: 2,
		Actions: []Action{RegD(HKLM, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Schedule\Maintenance`, "MaintenanceDisabled", 1)},
	},
	{
		ID: "ndu", Cat: "network", Group: "Background", Title: "Turn off the Network Data Usage monitor",
		Desc:    "Stops the driver that counts how much data each app uses.",
		Benefit: "Removes a driver that sits on every network packet and is known to slowly use more memory over time.",
		Note:    "Settings > Network > Data usage stops counting until you undo this.",
		Impact:  1, Risk: "moderate", Profile: 2, Reboot: true,
		Actions: []Action{RegD(HKLM, `SYSTEM\CurrentControlSet\Services\Ndu`, "Start", 4)},
	},
	{
		ID: "net-ecn", Cat: "network", Group: "Latency", Title: "Turn off ECN",
		Desc:    "Turns off Explicit Congestion Notification for TCP connections.",
		Benefit: "Avoids slow connections through older routers that mishandle ECN.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{Cmd("netsh int tcp set global ecncapability=disabled", "netsh int tcp set global ecncapability=default")},
	},
	{
		ID: "launcher-priority", Cat: "apps", Group: "Launchers", Title: "Run Steam and Epic store pages at lower priority",
		Desc:    "The Steam and Epic launcher web pages (steamwebhelper, EpicWebHelper) start at Below Normal priority.",
		Benefit: "Launchers left open in the background can't steal CPU time from your game.",
		Note:    "Games themselves are unaffected. Store pages might scroll a little less smoothly while a game is running.",
		Impact:  2, Risk: "safe", Profile: 2,
		Actions: []Action{
			RegD(HKLM, pIFEO+`\steamwebhelper.exe\PerfOptions`, "CpuPriorityClass", 5),
			RegD(HKLM, pIFEO+`\EpicWebHelper.exe\PerfOptions`, "CpuPriorityClass", 5),
		},
	},
	{
		ID: "svc-wisvc", Cat: "services", Group: "Unused features", Title: "Disable the Windows Insider service",
		Desc:    "Only needed if you're in the Windows Insider preview programme.",
		Benefit: "One less service running.",
		Note:    "If you get Insider preview builds, leave this off.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{Svc("wisvc", "disabled")},
	},
	{
		ID: "svc-lfsvc", Cat: "services", Group: "Unused features", Title: "Disable the Geolocation service",
		Desc:    "Stops apps asking Windows for your location.",
		Benefit: "One less service and fewer background location checks.",
		Note:    "Weather, Maps and Find My Device can't see your location until you undo this.",
		Impact:  1, Risk: "moderate", Profile: 2,
		Actions: []Action{Svc("lfsvc", "disabled")},
	},
}

func init() { Catalog = append(Catalog, extraTweaks...) }
