package main

// The full AstroSpeed tweak catalog.
//
// Ground rules this file must obey (enforced by catalog_test.go):
//   * Nothing touches mouse sensitivity, pointer speed, pointer precision, DPI,
//     mouse/keyboard driver buffers, display resolution or refresh rate.
//   * Nothing touches Fortnite's own settings (GameUserSettings.ini, keybinds,
//     anything under FortniteGame\Saved\Config) or the Epic launcher's settings.
//   * Every tweak is reversible: registry/service/task/bcd actions are backed up
//     automatically; command actions must provide a revert command.
//   * Anything that weakens security or changes how Windows looks is manual-only
//     (Profile 0) and never applied by the one-click Optimize button.

const (
	HKLM = "HKLM"
	HKCU = "HKCU"

	pGameCfg     = `System\GameConfigStore`
	pGameDVR     = `SOFTWARE\Microsoft\Windows\CurrentVersion\GameDVR`
	pGameDVRPol  = `SOFTWARE\Policies\Microsoft\Windows\GameDVR`
	pGameBar     = `Software\Microsoft\GameBar`
	pMMCSS       = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Multimedia\SystemProfile`
	pMMCSSGames  = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Multimedia\SystemProfile\Tasks\Games`
	pPriCtl      = `SYSTEM\CurrentControlSet\Control\PriorityControl`
	pMemMgmt     = `SYSTEM\CurrentControlSet\Control\Session Manager\Memory Management`
	pKernel      = `SYSTEM\CurrentControlSet\Control\Session Manager\kernel`
	pPowerThr    = `SYSTEM\CurrentControlSet\Control\Power\PowerThrottling`
	pSMPower     = `SYSTEM\CurrentControlSet\Control\Session Manager\Power`
	pGfxDrv      = `SYSTEM\CurrentControlSet\Control\GraphicsDrivers`
	pDwm         = `SOFTWARE\Microsoft\Windows\Dwm`
	pTcpip       = `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters`
	pTcpQoS      = `SYSTEM\CurrentControlSet\Services\Tcpip\QoS`
	pPsched      = `SOFTWARE\Policies\Microsoft\Windows\Psched`
	pQoSPol      = `SOFTWARE\Policies\Microsoft\Windows\QoS`
	pIFEO        = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options`
	pDataColl    = `SOFTWARE\Policies\Microsoft\Windows\DataCollection`
	pDataCollU   = `SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\DataCollection`
	pCDM         = `Software\Microsoft\Windows\CurrentVersion\ContentDeliveryManager`
	pSysPol      = `SOFTWARE\Policies\Microsoft\Windows\System`
	pExplAdv     = `Software\Microsoft\Windows\CurrentVersion\Explorer\Advanced`
	pEdgePol     = `SOFTWARE\Policies\Microsoft\Edge`
	pChromePol   = `SOFTWARE\Policies\Google\Chrome`
	pBravePol    = `SOFTWARE\Policies\BraveSoftware\Brave`
	pWUPol       = `SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`
	pDOPol       = `SOFTWARE\Policies\Microsoft\Windows\DeliveryOptimization`
	pWER         = `SOFTWARE\Microsoft\Windows\Windows Error Reporting`
	pFS          = `SYSTEM\CurrentControlSet\Control\FileSystem`
	pDevGuard    = `SYSTEM\CurrentControlSet\Control\DeviceGuard`
	pHVCI        = `SYSTEM\CurrentControlSet\Control\DeviceGuard\Scenarios\HypervisorEnforcedCodeIntegrity`
	pFnExe       = `FortniteClient-Win64-Shipping.exe`
	pSearchPol   = `SOFTWARE\Policies\Microsoft\Windows\Windows Search`
	pAppPrivacy  = `SOFTWARE\Policies\Microsoft\Windows\AppPrivacy`
	pCloudPol    = `SOFTWARE\Policies\Microsoft\Windows\CloudContent`
	pExplPolU    = `Software\Policies\Microsoft\Windows\Explorer`
	pPersonalize = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
)

var Categories = []Category{
	{ID: "fortnite", Title: "Fortnite", Subtitle: "Priority, GPU choice, fullscreen and network marking for the Fortnite process only. Your in-game settings and keybinds are never opened.", Icon: "fortnite", Game: "fortnite"},
	{ID: "gaming", Title: "Windows gaming", Subtitle: "Game Mode, background recording, fullscreen and windowed-game behaviour.", Icon: "gamepad", Nav: "Gaming"},
	{ID: "cpu", Title: "CPU & scheduler", Subtitle: "How Windows shares CPU time between the game and everything else.", Icon: "cpu", Nav: "CPU"},
	{ID: "power", Title: "Power", Subtitle: "Stops Windows clocking down your CPU, PCIe and USB to save power while you play.", Icon: "bolt"},
	{ID: "network", Title: "Network", Subtitle: "Fewer buffering and power-saving delays between your PC and the game server.", Icon: "wifi"},
	{ID: "gpu", Title: "Graphics", Subtitle: "GPU scheduling, presentation mode and driver background tasks.", Icon: "gpu"},
	{ID: "input", Title: "Input & USB", Subtitle: "Keeps USB ports and controllers awake. Sensitivity, DPI and pointer settings are never touched.", Icon: "mouse", Nav: "Input"},
	{ID: "apps", Title: "Discord & apps", Subtitle: "Discord voice priority, hardware acceleration and browsers running in the background.", Icon: "chat", Nav: "Apps"},
	{ID: "background", Title: "Background & privacy", Subtitle: "Telemetry, tips, suggestions and update traffic that run while you play.", Icon: "eye", Nav: "Privacy"},
	{ID: "services", Title: "Services", Subtitle: "Windows services and scheduled tasks you are unlikely to need on a gaming PC.", Icon: "layers"},
	{ID: "kernel", Title: "BIOS & kernel", Subtitle: "Boot-level timer settings, security trade-offs and a BIOS checklist.", Icon: "chip", Nav: "BIOS"},
	{ID: "appearance", Title: "Appearance", Subtitle: "Optional visual changes. These change how Windows looks, so they are never applied automatically.", Icon: "palette"},
}

func fnIFEO() string { return pIFEO + `\` + pFnExe + `\PerfOptions` }

var Catalog = []Tweak{

	// ────────────────────────────── FORTNITE ──────────────────────────────
	{
		ID: "fn-priority", Cat: "fortnite", Group: "Process", Title: "Run Fortnite at high CPU priority",
		Desc:    "Every time Fortnite starts, Windows gives it High priority instead of Normal, so background apps wait their turn.",
		Benefit: "Fewer frame-time spikes when something in the background wakes up.",
		Impact:  2, Risk: "safe", Profile: 2,
		Actions: []Action{RegD(HKLM, fnIFEO(), "CpuPriorityClass", 3)},
	},
	{
		ID: "fn-iopriority", Cat: "fortnite", Group: "Process", Title: "Give Fortnite high disk priority",
		Desc:    "Fortnite's file reads (textures, map streaming) jump ahead of background disk activity.",
		Benefit: "Smoother loading of new areas when landing.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{RegD(HKLM, fnIFEO(), "IoPriority", 3)},
	},
	{
		ID: "fn-gpupref", Cat: "fortnite", Group: "Process", Title: "Always use your high-performance GPU for Fortnite",
		Desc:    "Tells Windows to run Fortnite on your dedicated graphics card, never on integrated graphics.",
		Benefit: "Fixes laptops and some desktops silently running Fortnite on the slow GPU.",
		Impact:  3, Risk: "safe", Profile: 1, When: []string{"fortnite"},
		Actions: []Action{Dyn("fn_gpupref", "")},
	},
	{
		ID: "fn-fso", Cat: "fortnite", Group: "Process", Title: "Disable fullscreen optimizations for Fortnite",
		Desc:    "Same as ticking \"Disable fullscreen optimizations\" in Fortnite's exe properties. Only affects the Fortnite exe.",
		Benefit: "True exclusive fullscreen, which some players find lower latency.",
		Note:    "If you play in Windowed Fullscreen this makes no difference.",
		Impact:  1, Risk: "safe", Profile: 2, When: []string{"fortnite"},
		Actions: []Action{Dyn("fn_fso", "")},
	},
	{
		ID: "fn-qos", Cat: "fortnite", Group: "Network", Title: "Mark Fortnite traffic as high priority (DSCP 46)",
		Desc:    "Adds a Windows QoS policy that tags Fortnite's game packets as real-time traffic.",
		Benefit: "Routers that honour QoS tags send Fortnite packets first when someone else is streaming or downloading.",
		Note:    "Many home routers ignore DSCP tags. Harmless if yours does.",
		Impact:  1, Risk: "safe", Profile: 1, Reboot: true,
		Actions: []Action{
			RegS(HKLM, pTcpQoS, "Do not use NLA", "1"),
			Custom("qos_policy", "AstroSpeed Fortnite|FortniteClient-Win64-Shipping.exe|46"),
		},
	},

	// ────────────────────────────── WINDOWS GAMING ──────────────────────────────
	{
		ID: "gamemode", Cat: "gaming", Group: "Game Mode", Title: "Turn on Game Mode",
		Desc:    "Windows Game Mode stops Windows Update installing drivers and restarting, and prioritises the game.",
		Benefit: "Fewer background interruptions mid-match.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{
			RegD(HKCU, pGameBar, "AutoGameModeEnabled", 1),
			RegD(HKCU, pGameBar, "AllowAutoGameMode", 1),
		},
	},
	{
		ID: "gamedvr", Cat: "gaming", Group: "Recording", Title: "Turn off Game DVR background recording",
		Desc:    "Stops Windows constantly recording the last 30 seconds of your gameplay in the background.",
		Benefit: "Frees GPU encoder time and memory. One of the biggest single wins on older GPUs.",
		Impact:  3, Risk: "safe", Profile: 1,
		Actions: []Action{
			RegD(HKCU, pGameCfg, "GameDVR_Enabled", 0),
			RegD(HKLM, pGameDVRPol, "AllowGameDVR", 0),
			RegD(HKCU, pGameDVR, "AppCaptureEnabled", 0),
			RegD(HKCU, pGameDVR, "HistoricalCaptureEnabled", 0),
		},
	},
	{
		ID: "gamebar-capture", Cat: "gaming", Group: "Recording", Title: "Turn off Game Bar audio and cursor capture",
		Desc:    "Disables the extra audio and cursor capture hooks Game Bar keeps ready during games.",
		Benefit: "One less overlay hook inside the game process.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{
			RegD(HKCU, pGameDVR, "AudioCaptureEnabled", 0),
			RegD(HKCU, pGameDVR, "CursorCaptureEnabled", 0),
			RegD(HKCU, pGameDVR, "MicrophoneCaptureEnabled", 0),
			RegD(HKCU, pGameBar, "ShowStartupPanel", 0),
		},
	},
	{
		ID: "gamebar-controller", Cat: "gaming", Group: "Recording", Title: "Stop the Xbox button opening Game Bar",
		Desc:    "Pressing the Xbox/Guide button on a controller will no longer pop Game Bar over your game.",
		Benefit: "No accidental overlay mid-fight for controller players.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{RegD(HKCU, pGameBar, "UseNexusForGameBarEnabled", 0)},
	},
	{
		ID: "fso-global", Cat: "gaming", Group: "Fullscreen", Title: "Prefer exclusive fullscreen in all games",
		Desc:    "Makes \"Fullscreen\" in games mean real exclusive fullscreen instead of Windows' optimised borderless mode.",
		Benefit: "The game owns the display directly, skipping the desktop compositor.",
		Note:    "Can disable Auto HDR in exclusive fullscreen games.",
		Impact:  2, Risk: "moderate", Profile: 2,
		Actions: []Action{
			RegD(HKCU, pGameCfg, "GameDVR_FSEBehaviorMode", 2),
			RegD(HKCU, pGameCfg, "GameDVR_FSEBehavior", 2),
			RegD(HKCU, pGameCfg, "GameDVR_HonorUserFSEBehaviorMode", 1),
			RegD(HKCU, pGameCfg, "GameDVR_DXGIHonorFSEWindowsCompatible", 1),
			RegD(HKCU, pGameCfg, "GameDVR_EFSEFeatureFlags", 0),
		},
	},
	{
		ID: "windowed-opt", Cat: "gaming", Group: "Fullscreen", Title: "Turn on optimizations for windowed games",
		Desc:    "Lets DirectX 10/11 games running windowed or borderless use the faster flip presentation model.",
		Benefit: "Lower latency in Windowed Fullscreen, close to real fullscreen.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{Dyn("swapeffect", "")},
	},
	{
		ID: "power-throttling", Cat: "gaming", Group: "Background", Title: "Turn off power throttling",
		Desc:    "Stops Windows pushing apps it thinks are in the background onto slow, efficient CPU states.",
		Benefit: "Discord, OBS and your game never get mistaken for idle apps.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{RegD(HKLM, pPowerThr, "PowerThrottlingOff", 1)},
	},
	{
		ID: "drivers-wu", Cat: "gaming", Group: "Updates", Title: "Stop Windows Update replacing your GPU driver",
		Desc:    "Windows Update stops installing driver updates, so the GPU driver you picked stays put.",
		Benefit: "No surprise driver downgrades that break performance.",
		Note:    "Update drivers yourself from NVIDIA, AMD or Intel.",
		Impact:  1, Risk: "moderate", Profile: 2,
		Actions: []Action{RegD(HKLM, pWUPol, "ExcludeWUDriversInQualityUpdate", 1)},
	},
	{
		ID: "fast-startup", Cat: "gaming", Group: "Startup", Title: "Turn off Fast Startup",
		Desc:    "Shutting down now fully restarts the Windows kernel instead of half-hibernating it.",
		Benefit: "Every boot starts clean, so timer and driver tweaks actually apply and memory leaks don't carry over.",
		Impact:  1, Risk: "safe", Profile: 1, Reboot: true,
		Actions: []Action{RegD(HKLM, pSMPower, "HiberbootEnabled", 0)},
	},
	{
		ID: "startup-delay", Cat: "gaming", Group: "Startup", Title: "Remove the startup app delay",
		Desc:    "Windows normally waits several seconds after login before starting apps. This removes the wait.",
		Benefit: "Desktop is ready sooner after a restart.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{RegD(HKCU, `Software\Microsoft\Windows\CurrentVersion\Explorer\Serialize`, "StartupDelayInMSec", 0)},
	},
	{
		ID: "hibernation", Cat: "gaming", Group: "Startup", Title: "Turn off hibernation",
		Desc:    "Deletes hiberfil.sys and frees disk space equal to a large part of your RAM.",
		Benefit: "Several GB of SSD space back.",
		Note:    "Laptops lose the Hibernate option. Manual only.",
		Impact:  1, Risk: "moderate", Profile: 0,
		Actions: []Action{Cmd("powercfg /hibernate off", "powercfg /hibernate on")},
	},

	// ────────────────────────────── CPU & SCHEDULER ──────────────────────────────
	{
		ID: "mmcss-responsiveness", Cat: "cpu", Group: "Multimedia scheduler", Title: "Reserve less CPU for background tasks",
		Desc:    "Windows normally reserves 20% of CPU time for low-priority work. This lowers it to the minimum of 10%.",
		Benefit: "More CPU time available to the game.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{RegD(HKLM, pMMCSS, "SystemResponsiveness", 10)},
	},
	{
		ID: "mmcss-games", Cat: "cpu", Group: "Multimedia scheduler", Title: "Give games top priority in the multimedia scheduler",
		Desc:    "Raises the Games profile to the highest GPU and CPU priority the Windows multimedia scheduler allows.",
		Benefit: "Game threads registered with MMCSS get scheduled ahead of other media work.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{
			RegD(HKLM, pMMCSSGames, "GPU Priority", 8),
			RegD(HKLM, pMMCSSGames, "Priority", 6),
			RegS(HKLM, pMMCSSGames, "Scheduling Category", "High"),
			RegS(HKLM, pMMCSSGames, "SFIO Priority", "High"),
		},
	},
	{
		ID: "priority-sep", Cat: "cpu", Group: "Scheduler", Title: "Favour the foreground app (Win32PrioritySeparation 0x26)",
		Desc:    "Gives the window you're playing in short, variable time slices with the maximum foreground boost.",
		Benefit: "Game input and render threads get CPU back faster after background work.",
		Impact:  2, Risk: "safe", Profile: 2,
		Actions: []Action{RegD(HKLM, pPriCtl, "Win32PrioritySeparation", 0x26)},
	},
	{
		ID: "global-timer", Cat: "cpu", Group: "Timers", Title: "Allow global timer resolution requests",
		Desc:    "On Windows 11, a program asking for a 0.5 ms timer only gets it for itself. This restores the old global behaviour.",
		Benefit: "AstroSpeed's Game Booster timer (and similar tools) can lower timer latency system-wide again.",
		Impact:  1, Risk: "safe", Profile: 2, Reboot: true, When: []string{"win11"},
		Actions: []Action{RegD(HKLM, pKernel, "GlobalTimerResolutionRequests", 1)},
	},
	{
		ID: "paging-exec", Cat: "cpu", Group: "Memory", Title: "Keep the Windows kernel in RAM",
		Desc:    "Stops Windows paging kernel and driver code out to disk.",
		Benefit: "Kernel and driver code is always in memory, avoiding rare disk-read hitches.",
		Impact:  1, Risk: "safe", Profile: 2, Reboot: true, When: []string{"ram16"},
		Actions: []Action{RegD(HKLM, pMemMgmt, "DisablePagingExecutive", 1)},
	},
	{
		ID: "mem-compression", Cat: "cpu", Group: "Memory", Title: "Turn off memory compression",
		Desc:    "Windows stops spending CPU cycles compressing RAM it thinks you're not using.",
		Benefit: "Frees CPU time on PCs with plenty of RAM.",
		Note:    "Only offered on PCs with 16 GB of RAM or more.",
		Impact:  1, Risk: "moderate", Profile: 2, Reboot: true, When: []string{"ram16"},
		Actions: []Action{Custom("mmagent", "MemoryCompression")},
	},
	{
		ID: "page-combining", Cat: "cpu", Group: "Memory", Title: "Turn off page combining",
		Desc:    "Stops Windows scanning RAM for duplicate pages to merge.",
		Benefit: "One less periodic background memory scan.",
		Impact:  1, Risk: "moderate", Profile: 2, Reboot: true, When: []string{"ram16"},
		Actions: []Action{Custom("mmagent", "PageCombining")},
	},
	{
		ID: "svchost-split", Cat: "cpu", Group: "Memory", Title: "Group Windows services into fewer processes",
		Desc:    "Sets the svchost split threshold to your RAM size so Windows runs services in shared processes.",
		Benefit: "Noticeably fewer background processes in Task Manager.",
		Impact:  1, Risk: "moderate", Profile: 2, Reboot: true, When: []string{"ram16"},
		Actions: []Action{Dyn("svchostsplit", "")},
	},
	{
		ID: "ntfs-lastaccess", Cat: "cpu", Group: "Disk", Title: "Stop updating file \"last accessed\" times",
		Desc:    "NTFS stops writing a timestamp every time any file is read.",
		Benefit: "Fewer small disk writes while games stream assets.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{RegD(HKLM, pFS, "NtfsDisableLastAccessUpdate", 0x80000001)},
	},
	{
		ID: "ntfs-8dot3", Cat: "cpu", Group: "Disk", Title: "Stop creating legacy 8.3 short file names",
		Desc:    "New files no longer get a second DOS-style short name.",
		Benefit: "Slightly faster file creation in big folders.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{RegD(HKLM, pFS, "NtfsDisable8dot3NameCreation", 1)},
	},

	// ────────────────────────────── POWER ──────────────────────────────
	{
		ID: "power-plan", Cat: "power", Group: "Power plan", Title: "Switch to the AstroSpeed Performance power plan",
		Desc:    "Creates a plan based on Ultimate Performance: CPU never clocks down, PCIe and USB never sleep, Wi-Fi at full power. Applies when plugged in only.",
		Benefit: "No clock-speed dips or wake-up delays between fights.",
		Note:    "Uses more power at idle. Battery settings are not changed.",
		Impact:  3, Risk: "safe", Profile: 1,
		Actions: []Action{Custom("powerplan", "")},
	},
	{
		ID: "core-parking", Cat: "power", Group: "CPU", Title: "Disable CPU core parking",
		Desc:    "Keeps every CPU core awake instead of letting Windows park idle cores.",
		Benefit: "Cores never have to wake up mid-fight.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{Custom("coreparking", "")},
	},
	{
		ID: "usb-suspend", Cat: "power", Group: "USB", Title: "Turn off USB selective suspend",
		Desc:    "Windows stops putting idle USB ports to sleep.",
		Benefit: "Mouse, keyboard and headset never need to wake up, avoiding first-input lag and disconnects.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{RegD(HKLM, `SYSTEM\CurrentControlSet\Services\USB`, "DisableSelectiveSuspend", 1)},
	},
	{
		ID: "idle-disable", Cat: "power", Group: "CPU", Title: "Disable CPU idle states",
		Desc:    "CPU cores run at 100% all the time and never enter sleep states.",
		Benefit: "Removes idle-exit latency completely.",
		Note:    "Much higher temperatures, fan noise and power use. Can lower boost clocks on some CPUs. Manual only.",
		Impact:  2, Risk: "expert", Profile: 0,
		Actions: []Action{Custom("idledisable", "")},
	},

	// ────────────────────────────── NETWORK ──────────────────────────────
	{
		ID: "net-throttling", Cat: "network", Group: "Throughput", Title: "Turn off network throttling",
		Desc:    "Windows limits network packet processing while media plays. This removes that limit.",
		Benefit: "Game packets aren't held back when Discord or music is playing.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{RegD(HKLM, pMMCSS, "NetworkThrottlingIndex", 0xFFFFFFFF)},
	},
	{
		ID: "nagle", Cat: "network", Group: "Latency", Title: "Turn off Nagle's algorithm",
		Desc:    "Small TCP packets are sent immediately instead of being bundled and delayed.",
		Benefit: "Lower delay for TCP traffic such as voice chat signalling and lobby/party services.",
		Note:    "Fortnite gameplay itself uses UDP, so the in-match effect is small.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{Dyn("nagle", "")},
	},
	{
		ID: "qos-reserve", Cat: "network", Group: "Throughput", Title: "Remove the QoS bandwidth reservation",
		Desc:    "Sets the QoS Packet Scheduler's reservable bandwidth to 0%.",
		Benefit: "Guarantees no bandwidth is held back for QoS-aware apps.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{RegD(HKLM, pPsched, "NonBestEffortLimit", 0)},
	},
	{
		ID: "nic-power", Cat: "network", Group: "Adapter", Title: "Turn off network adapter power saving",
		Desc:    "Disables Energy-Efficient Ethernet, Green Ethernet, ultra-low-power mode and \"turn off this device to save power\" on every adapter.",
		Benefit: "Your network card never naps between packets, which causes ping spikes.",
		Impact:  3, Risk: "safe", Profile: 1, Reboot: true,
		Actions: []Action{Custom("nicpower", "")},
	},
	{
		ID: "nic-interrupt", Cat: "network", Group: "Adapter", Title: "Turn off interrupt moderation",
		Desc:    "The network card signals every packet immediately instead of batching them.",
		Benefit: "Lowest possible packet latency.",
		Note:    "Slightly higher CPU use on very fast downloads.",
		Impact:  2, Risk: "moderate", Profile: 2, Reboot: true,
		Actions: []Action{Custom("nicadv", "*InterruptModeration=0")},
	},
	{
		ID: "nic-flow", Cat: "network", Group: "Adapter", Title: "Turn off Ethernet flow control",
		Desc:    "Stops the adapter pausing traffic when the switch or router signals it is busy.",
		Benefit: "Avoids pause frames delaying game packets.",
		Impact:  1, Risk: "moderate", Profile: 2, Reboot: true,
		Actions: []Action{Custom("nicadv", "*FlowControl=0")},
	},
	{
		ID: "nic-lso", Cat: "network", Group: "Adapter", Title: "Turn off Large Send Offload",
		Desc:    "Stops the adapter buffering outgoing data into large chunks before sending.",
		Benefit: "Outgoing packets leave immediately.",
		Impact:  1, Risk: "moderate", Profile: 2, Reboot: true,
		Actions: []Action{Custom("nicadv", "*LsoV2IPv4=0,*LsoV2IPv6=0")},
	},
	{
		ID: "tcp-rsc", Cat: "network", Group: "Latency", Title: "Turn off Receive Segment Coalescing",
		Desc:    "Incoming packets are handed to apps one by one instead of being merged first.",
		Benefit: "Removes a small receive-side delay.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{Cmd("netsh int tcp set global rsc=disabled", "netsh int tcp set global rsc=enabled")},
	},
	{
		ID: "tcp-rss", Cat: "network", Group: "Latency", Title: "Make sure Receive Side Scaling is on",
		Desc:    "Spreads network processing across CPU cores instead of piling it onto one.",
		Benefit: "Network work doesn't compete with the game's main thread.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{Cmd("netsh int tcp set global rss=enabled", "")},
	},
	{
		ID: "tcp-autotune", Cat: "network", Group: "Throughput", Title: "Reset TCP auto-tuning to normal",
		Desc:    "Restores Windows' normal TCP receive window tuning in case another tool restricted it.",
		Benefit: "Full download speed for updates and patches.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{Cmd("netsh int tcp set global autotuninglevel=normal", "")},
	},
	{
		ID: "teredo", Cat: "network", Group: "Latency", Title: "Turn off Teredo tunnelling",
		Desc:    "Disables the old IPv6-over-IPv4 tunnel adapter.",
		Benefit: "One less virtual adapter doing background work.",
		Note:    "Xbox app party chat needs Teredo. Manual only.",
		Impact:  1, Risk: "moderate", Profile: 0,
		Actions: []Action{Cmd("netsh interface teredo set state disabled", "netsh interface teredo set state default")},
	},

	// ────────────────────────────── GRAPHICS ──────────────────────────────
	{
		ID: "hags", Cat: "gpu", Group: "Scheduling", Title: "Turn on hardware-accelerated GPU scheduling",
		Desc:    "Lets your graphics card manage its own memory and work queue instead of the CPU doing it.",
		Benefit: "Less CPU overhead per frame; needed for DLSS Frame Generation.",
		Impact:  2, Risk: "safe", Profile: 2, Reboot: true,
		Actions: []Action{RegD(HKLM, pGfxDrv, "HwSchMode", 2)},
	},
	{
		ID: "mpo", Cat: "gpu", Group: "Scheduling", Title: "Turn off multiplane overlay (MPO)",
		Desc:    "Disables MPO, a display feature known to cause flicker and stutter on some NVIDIA setups.",
		Benefit: "Fixes flicker, black screens and stutter if you have them.",
		Note:    "Only use this if you actually see flicker or stutter. It can raise latency in windowed mode. Manual only.",
		Impact:  1, Risk: "moderate", Profile: 0, Reboot: true,
		Actions: []Action{RegD(HKLM, pDwm, "OverlayTestMode", 5)},
	},
	{
		ID: "gpu-msi", Cat: "gpu", Group: "Interrupts", Title: "Put your GPU in MSI interrupt mode",
		Desc:    "Switches the graphics card from shared line-based interrupts to message-signalled interrupts.",
		Benefit: "Lower interrupt latency if your GPU was still using legacy interrupts.",
		Note:    "Most modern GPUs already use MSI. Revert if you see a black screen after reboot. Manual only.",
		Impact:  1, Risk: "expert", Profile: 0, Reboot: true,
		Actions: []Action{Dyn("gpu_msi", "")},
	},
	{
		ID: "nv-telemetry", Cat: "gpu", Group: "Driver background", Title: "Turn off NVIDIA telemetry tasks",
		Desc:    "Disables NVIDIA's crash-report and telemetry scheduled tasks.",
		Benefit: "Stops NVIDIA background reporting firing during play.",
		Impact:  1, Risk: "safe", Profile: 1, When: []string{"nvidia"},
		Actions: []Action{
			Task(`NvTmRep*`),
			Task(`NvTmMon*`),
			Task(`NvProfileUpdater*`),
			Svc("NvTelemetryContainer", "disabled"),
		},
	},
	{
		ID: "nv-opt-in", Cat: "gpu", Group: "Driver background", Title: "Opt out of NVIDIA data collection",
		Desc:    "Sets NVIDIA's own opt-out flag for driver usage data.",
		Benefit: "Less background data collection from the NVIDIA driver.",
		Impact:  1, Risk: "safe", Profile: 1, When: []string{"nvidia"},
		Actions: []Action{RegD(HKLM, `SOFTWARE\NVIDIA Corporation\NvControlPanel2\Client`, "OptInOrOutPreference", 0)},
	},
	{
		ID: "amd-ulps", Cat: "gpu", Group: "Driver background", Title: "Turn off AMD ultra-low power state",
		Desc:    "Stops AMD graphics cards dropping into ULPS when idle.",
		Benefit: "Avoids wake-up stutters on AMD cards, especially multi-monitor setups.",
		Impact:  1, Risk: "moderate", Profile: 2, Reboot: true, When: []string{"amd"},
		Actions: []Action{Dyn("amd_ulps", "")},
	},

	// ────────────────────────────── INPUT & USB ──────────────────────────────
	{
		ID: "usb-devpower", Cat: "input", Group: "USB", Title: "Stop Windows turning off USB devices to save power",
		Desc:    "Unticks \"Allow the computer to turn off this device to save power\" on every USB hub, controller and HID device.",
		Benefit: "No random mouse or keyboard sleep, disconnects or first-click delay.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{Custom("usbpower", "")},
	},
	{
		ID: "access-shortcuts", Cat: "input", Group: "Keyboard", Title: "Stop the Sticky Keys pop-up when you spam Shift",
		Desc:    "Turns off only the keyboard shortcuts that open Sticky, Toggle and Filter Keys. The features themselves are left exactly as they are.",
		Benefit: "No more getting tabbed out of a fight by the Sticky Keys box.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{Dyn("accesskeys", "")},
	},

	// ────────────────────────────── DISCORD & APPS ──────────────────────────────
	{
		ID: "discord-qos", Cat: "apps", Group: "Discord", Title: "Neutralise Discord's high packet priority (QoS)",
		Desc:    "Adds a Windows QoS policy that strips Discord's \"high priority\" packet tag, so voice packets no longer compete with Fortnite's.",
		Benefit: "Stops Discord voice getting ahead of game packets on routers that honour QoS. Also turn off the switch inside Discord (see steps above).",
		Impact:  2, Risk: "safe", Profile: 1, Reboot: true,
		Actions: []Action{
			RegS(HKLM, pTcpQoS, "Do not use NLA", "1"),
			Custom("qos_policy", "AstroSpeed Discord|Discord.exe|0"),
		},
	},
	{
		ID: "discord-hwaccel", Cat: "apps", Group: "Discord", Title: "Turn off Discord hardware acceleration",
		Desc:    "Discord stops using your GPU to draw its window.",
		Benefit: "Discord no longer competes with Fortnite for GPU time while you're in a call.",
		Note:    "Close Discord fully (tray icon → Quit) before applying, or Discord will overwrite the change.",
		Impact:  2, Risk: "safe", Profile: 2, When: []string{"discord"},
		Actions: []Action{Custom("discord_hwaccel", "")},
	},
	{
		ID: "edge-bg", Cat: "apps", Group: "Browsers", Title: "Stop Microsoft Edge running in the background",
		Desc:    "Turns off Edge Startup Boost and background mode, so Edge only runs when you open it.",
		Benefit: "Frees hundreds of MB of RAM and background CPU.",
		Note:    "Edge will show \"Managed by your organization\" in its settings. That's just this policy.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{
			RegD(HKLM, pEdgePol, "StartupBoostEnabled", 0),
			RegD(HKLM, pEdgePol, "BackgroundModeEnabled", 0),
		},
	},
	{
		ID: "chrome-bg", Cat: "apps", Group: "Browsers", Title: "Stop Chrome and Brave running in the background",
		Desc:    "Closing the browser window actually closes the browser.",
		Benefit: "No hidden browser processes using RAM and CPU while you play.",
		Note:    "Chrome/Brave will show \"Managed by your organization\". That's just this policy.",
		Impact:  2, Risk: "safe", Profile: 2,
		Actions: []Action{
			RegD(HKLM, pChromePol, "BackgroundModeEnabled", 0),
			RegD(HKLM, pBravePol, "BackgroundModeEnabled", 0),
		},
	},
	{
		ID: "edge-update-svc", Cat: "apps", Group: "Updaters", Title: "Set browser updater services to manual",
		Desc:    "Edge and Google updater services start only when needed instead of with Windows.",
		Benefit: "Fewer background services at boot. Browsers still update themselves when opened.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{
			Svc("edgeupdate", "manual"), Svc("edgeupdatem", "manual"),
			Svc("gupdate", "manual"), Svc("gupdatem", "manual"),
			Svc("GoogleUpdaterService", "manual"), Svc("GoogleUpdaterInternalService", "manual"),
			Svc("brave", "manual"), Svc("bravem", "manual"),
		},
	},
	{
		ID: "adobe-svc", Cat: "apps", Group: "Updaters", Title: "Set Adobe updater services to manual",
		Desc:    "Adobe's update and licensing helpers start only when an Adobe app needs them.",
		Benefit: "Fewer always-running background services.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{Svc("AdobeARMservice", "manual"), Svc("AGSService", "manual"), Svc("AdobeUpdateService", "manual")},
	},

	// ────────────────────────────── BACKGROUND & PRIVACY ──────────────────────────────
	{
		ID: "telemetry", Cat: "background", Group: "Telemetry", Title: "Turn diagnostic data down to the minimum",
		Desc:    "Sets Windows diagnostic data to the lowest level your edition allows.",
		Benefit: "Less background data collection and upload.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{
			RegD(HKLM, pDataColl, "AllowTelemetry", 0),
			RegD(HKLM, pDataCollU, "AllowTelemetry", 0),
			RegD(HKLM, pDataColl, "DoNotShowFeedbackNotifications", 1),
		},
	},
	{
		ID: "telemetry-svc", Cat: "background", Group: "Telemetry", Title: "Turn off the telemetry services",
		Desc:    "Disables Connected User Experiences and Telemetry, and the WAP push routing service it uses.",
		Benefit: "The telemetry service can spike disk and CPU while uploading.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{Svc("DiagTrack", "disabled"), Svc("dmwappushservice", "disabled")},
	},
	{
		ID: "telemetry-tasks", Cat: "background", Group: "Telemetry", Title: "Turn off telemetry scheduled tasks",
		Desc:    "Disables the compatibility appraiser, customer experience programme and disk diagnostic data tasks.",
		Benefit: "These tasks are known for random disk and CPU spikes, sometimes mid-game.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{
			Task(`\Microsoft\Windows\Application Experience\Microsoft Compatibility Appraiser`),
			Task(`\Microsoft\Windows\Application Experience\Microsoft Compatibility Appraiser Exp`),
			Task(`\Microsoft\Windows\Application Experience\ProgramDataUpdater`),
			Task(`\Microsoft\Windows\Application Experience\MareBackup`),
			Task(`\Microsoft\Windows\Application Experience\PcaPatchDbTask`),
			Task(`\Microsoft\Windows\Customer Experience Improvement Program\Consolidator`),
			Task(`\Microsoft\Windows\Customer Experience Improvement Program\UsbCeip`),
			Task(`\Microsoft\Windows\Autochk\Proxy`),
			Task(`\Microsoft\Windows\DiskDiagnostic\Microsoft-Windows-DiskDiagnosticDataCollector`),
			Task(`\Microsoft\Windows\Feedback\Siuf\DmClient`),
			Task(`\Microsoft\Windows\Feedback\Siuf\DmClientOnScenarioDownload`),
			Task(`\Microsoft\Windows\Windows Error Reporting\QueueReporting`),
			Task(`\Microsoft\Windows\Maps\MapsUpdateTask`),
			Task(`\Microsoft\Windows\Maps\MapsToastTask`),
		},
	},
	{
		ID: "error-reporting", Cat: "background", Group: "Telemetry", Title: "Turn off Windows Error Reporting",
		Desc:    "Crashes are no longer collected and uploaded to Microsoft.",
		Benefit: "No heavy dump collection running after an app crashes.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{
			RegD(HKLM, pWER, "Disabled", 1),
			RegD(HKLM, `SOFTWARE\Policies\Microsoft\Windows\Windows Error Reporting`, "Disabled", 1),
		},
	},
	{
		ID: "feedback", Cat: "background", Group: "Telemetry", Title: "Stop Windows asking for feedback",
		Desc:    "Sets Windows feedback prompts to Never.",
		Benefit: "No feedback pop-ups during play.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{
			RegD(HKCU, `Software\Microsoft\Siuf\Rules`, "NumberOfSIUFInPeriod", 0),
			RegD(HKCU, `Software\Microsoft\Siuf\Rules`, "PeriodInNanoSeconds", 0),
		},
	},
	{
		ID: "ad-id", Cat: "background", Group: "Privacy", Title: "Turn off the advertising ID",
		Desc:    "Apps can no longer use a Windows ID to personalise ads.",
		Benefit: "Privacy.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{
			RegD(HKCU, `Software\Microsoft\Windows\CurrentVersion\AdvertisingInfo`, "Enabled", 0),
			RegD(HKLM, `SOFTWARE\Policies\Microsoft\Windows\AdvertisingInfo`, "DisabledByGroupPolicy", 1),
		},
	},
	{
		ID: "tailored", Cat: "background", Group: "Privacy", Title: "Turn off tailored experiences",
		Desc:    "Microsoft stops using diagnostic data to personalise tips and ads.",
		Benefit: "Privacy.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{
			RegD(HKCU, `Software\Microsoft\Windows\CurrentVersion\Privacy`, "TailoredExperiencesWithDiagnosticDataEnabled", 0),
			RegD(HKCU, `Software\Policies\Microsoft\Windows\CloudContent`, "DisableTailoredExperiencesWithDiagnosticData", 1),
		},
	},
	{
		ID: "activity", Cat: "background", Group: "Privacy", Title: "Turn off activity history",
		Desc:    "Windows stops recording and uploading what apps and files you use.",
		Benefit: "Less background logging.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{
			RegD(HKLM, pSysPol, "EnableActivityFeed", 0),
			RegD(HKLM, pSysPol, "PublishUserActivities", 0),
			RegD(HKLM, pSysPol, "UploadUserActivities", 0),
		},
	},
	{
		ID: "tips", Cat: "background", Group: "Suggestions", Title: "Turn off Windows tips, suggestions and silent app installs",
		Desc:    "Stops tips, \"get even more out of Windows\" prompts, Start suggestions and automatic installs of promoted apps.",
		Benefit: "No pop-ups and no surprise apps downloading in the background.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{
			RegD(HKCU, pCDM, "SubscribedContent-338388Enabled", 0),
			RegD(HKCU, pCDM, "SubscribedContent-338389Enabled", 0),
			RegD(HKCU, pCDM, "SubscribedContent-353694Enabled", 0),
			RegD(HKCU, pCDM, "SubscribedContent-353696Enabled", 0),
			RegD(HKCU, pCDM, "SubscribedContent-310093Enabled", 0),
			RegD(HKCU, pCDM, "SystemPaneSuggestionsEnabled", 0),
			RegD(HKCU, pCDM, "SoftLandingEnabled", 0),
			RegD(HKCU, pCDM, "SilentInstalledAppsEnabled", 0),
			RegD(HKCU, pCDM, "OemPreInstalledAppsEnabled", 0),
			RegD(HKCU, pCDM, "PreInstalledAppsEnabled", 0),
			RegD(HKCU, `Software\Microsoft\Windows\CurrentVersion\UserProfileEngagement`, "ScoobeSystemSettingEnabled", 0),
			RegD(HKLM, pCloudPol, "DisableWindowsConsumerFeatures", 1),
			RegD(HKLM, pCloudPol, "DisableSoftLanding", 1),
		},
	},
	{
		ID: "delivery-opt", Cat: "background", Group: "Updates", Title: "Stop uploading Windows updates to other PCs",
		Desc:    "Delivery Optimization stops sharing update files with other computers over the internet.",
		Benefit: "Your upload bandwidth isn't used by Windows while you play.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{RegD(HKLM, pDOPol, "DODownloadMode", 0)},
	},
	{
		ID: "recall", Cat: "background", Group: "AI features", Title: "Turn off Recall snapshots",
		Desc:    "Stops Windows Recall taking and analysing screenshots on Copilot+ PCs.",
		Benefit: "No constant screenshotting and indexing in the background.",
		Impact:  2, Risk: "safe", Profile: 1,
		Actions: []Action{
			RegD(HKLM, `SOFTWARE\Policies\Microsoft\Windows\WindowsAI`, "DisableAIDataAnalysis", 1),
			RegD(HKCU, `Software\Policies\Microsoft\Windows\WindowsAI`, "DisableAIDataAnalysis", 1),
		},
	},
	{
		ID: "copilot", Cat: "background", Group: "AI features", Title: "Turn off Windows Copilot",
		Desc:    "Removes the built-in Copilot sidebar.",
		Benefit: "One less background component.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{
			RegD(HKCU, `Software\Policies\Microsoft\Windows\WindowsCopilot`, "TurnOffWindowsCopilot", 1),
			RegD(HKLM, `SOFTWARE\Policies\Microsoft\Windows\WindowsCopilot`, "TurnOffWindowsCopilot", 1),
		},
	},
	{
		ID: "web-search", Cat: "background", Group: "Search", Title: "Remove web results from Start search",
		Desc:    "Start menu search shows only your apps, files and settings, not Bing results.",
		Benefit: "Faster Start search with no network calls.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{
			RegD(HKCU, pExplPolU, "DisableSearchBoxSuggestions", 1),
			RegD(HKLM, pSearchPol, "ConnectedSearchUseWeb", 0),
			RegD(HKLM, pSearchPol, "DisableWebSearch", 1),
		},
	},
	{
		ID: "cortana", Cat: "background", Group: "Search", Title: "Turn off Cortana",
		Desc:    "Disables Cortana through policy.",
		Benefit: "One less background assistant.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{RegD(HKLM, pSearchPol, "AllowCortana", 0)},
	},
	{
		ID: "widgets", Cat: "background", Group: "Suggestions", Title: "Turn off Widgets and news feed",
		Desc:    "Removes the Widgets board and its news feed, which keep a web view running in the background.",
		Benefit: "Frees RAM and background network use.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{RegD(HKLM, `SOFTWARE\Policies\Microsoft\Dsh`, "AllowNewsAndInterests", 0)},
	},
	{
		ID: "bg-apps", Cat: "background", Group: "Background apps", Title: "Stop Store apps running in the background",
		Desc:    "Microsoft Store apps can no longer run when you're not using them.",
		Benefit: "Frees CPU and RAM from apps you aren't using.",
		Note:    "Apps like WhatsApp, Phone Link and Mail stop sending notifications when closed. Manual only.",
		Impact:  2, Risk: "moderate", Profile: 0,
		Actions: []Action{RegD(HKLM, pAppPrivacy, "LetAppsRunInBackground", 2)},
	},
	{
		ID: "app-tracking", Cat: "background", Group: "Privacy", Title: "Stop tracking which apps you launch",
		Desc:    "Windows stops logging app launches to build its \"most used\" list.",
		Benefit: "Privacy, and one less write on every program start.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{RegD(HKCU, pExplAdv, "Start_TrackProgs", 0)},
	},
	{
		ID: "typing-data", Cat: "background", Group: "Privacy", Title: "Turn off inking and typing data collection",
		Desc:    "Windows stops collecting what you type and write to build a personal dictionary.",
		Benefit: "Privacy. Your keyboard settings themselves are not changed.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{
			RegD(HKCU, `Software\Microsoft\InputPersonalization`, "RestrictImplicitInkCollection", 1),
			RegD(HKCU, `Software\Microsoft\InputPersonalization`, "RestrictImplicitTextCollection", 1),
			RegD(HKCU, `Software\Microsoft\InputPersonalization\TrainedDataStore`, "HarvestContacts", 0),
			RegD(HKCU, `Software\Microsoft\Personalization\Settings`, "AcceptedPrivacyPolicy", 0),
		},
	},
	{
		ID: "lang-list", Cat: "background", Group: "Privacy", Title: "Don't share your language list with websites",
		Desc:    "Websites can no longer read your installed Windows languages.",
		Benefit: "Privacy.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{RegD(HKCU, `Control Panel\International\User Profile`, "HttpAcceptLanguageOptOut", 1)},
	},
	{
		ID: "remote-assist", Cat: "background", Group: "Privacy", Title: "Turn off Remote Assistance",
		Desc:    "Nobody can be invited to view or control your PC through Windows Remote Assistance.",
		Benefit: "Security, and one less listener.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{RegD(HKLM, `SYSTEM\CurrentControlSet\Control\Remote Assistance`, "fAllowToGetHelp", 0)},
	},

	// ────────────────────────────── SERVICES ──────────────────────────────
	{
		ID: "svc-maps", Cat: "services", Group: "Unused features", Title: "Disable Downloaded Maps Manager",
		Desc:    "Service for offline maps in the Windows Maps app.",
		Benefit: "One less service running.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{Svc("MapsBroker", "disabled")},
	},
	{
		ID: "svc-retail", Cat: "services", Group: "Unused features", Title: "Disable Retail Demo service",
		Desc:    "Only used by shop display PCs.",
		Benefit: "One less service.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{Svc("RetailDemo", "disabled")},
	},
	{
		ID: "svc-fax", Cat: "services", Group: "Unused features", Title: "Disable Fax service",
		Desc:    "Fax support.",
		Benefit: "One less service.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{Svc("Fax", "disabled")},
	},
	{
		ID: "svc-wmp", Cat: "services", Group: "Unused features", Title: "Disable Windows Media Player network sharing",
		Desc:    "Legacy media sharing over your home network.",
		Benefit: "One less network listener.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{Svc("WMPNetworkSvc", "disabled")},
	},
	{
		ID: "svc-remotereg", Cat: "services", Group: "Unused features", Title: "Disable Remote Registry",
		Desc:    "Lets other computers edit this PC's registry over the network.",
		Benefit: "Security.",
		Impact:  1, Risk: "safe", Profile: 1,
		Actions: []Action{Svc("RemoteRegistry", "disabled")},
	},
	{
		ID: "svc-pca", Cat: "services", Group: "Background helpers", Title: "Disable Program Compatibility Assistant",
		Desc:    "Watches programs you run for known compatibility problems.",
		Benefit: "Removes a hook that inspects every program launch.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{Svc("PcaSvc", "disabled")},
	},
	{
		ID: "svc-trkwks", Cat: "services", Group: "Background helpers", Title: "Disable Distributed Link Tracking",
		Desc:    "Tracks shortcut targets moved between NTFS drives on a network.",
		Benefit: "One less background service.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{Svc("TrkWks", "disabled")},
	},
	{
		ID: "svc-diaghub", Cat: "services", Group: "Background helpers", Title: "Disable Diagnostics Hub collector",
		Desc:    "Collects diagnostic traces for developer tools.",
		Benefit: "One less service.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{Svc("diagnosticshub.standardcollector.service", "disabled")},
	},
	{
		ID: "svc-wer", Cat: "services", Group: "Background helpers", Title: "Disable Windows Error Reporting service",
		Desc:    "The service that gathers crash reports.",
		Benefit: "No crash-dump processing in the background.",
		Impact:  1, Risk: "safe", Profile: 2,
		Actions: []Action{Svc("WerSvc", "disabled")},
	},
	{
		ID: "svc-sysmain", Cat: "services", Group: "Optional", Title: "Disable SysMain (Superfetch)",
		Desc:    "Stops Windows preloading apps it predicts you'll open.",
		Benefit: "Removes background disk and RAM churn on fast SSDs.",
		Note:    "On hard drives this makes apps open slower. Manual only.",
		Impact:  1, Risk: "moderate", Profile: 0,
		Actions: []Action{Svc("SysMain", "disabled")},
	},
	{
		ID: "svc-search", Cat: "services", Group: "Optional", Title: "Disable Windows Search indexing",
		Desc:    "Stops the indexer that scans your files for fast Start-menu search.",
		Benefit: "No indexer disk activity after downloads and updates.",
		Note:    "File search in Start and Explorer becomes slower. Manual only.",
		Impact:  1, Risk: "moderate", Profile: 0,
		Actions: []Action{Svc("WSearch", "disabled")},
	},
	{
		ID: "svc-spooler", Cat: "services", Group: "Optional", Title: "Disable Print Spooler",
		Desc:    "Turns off printing support.",
		Benefit: "One less service, and closes a well-known attack surface.",
		Note:    "You won't be able to print. Manual only.",
		Impact:  1, Risk: "moderate", Profile: 0,
		Actions: []Action{Svc("Spooler", "disabled")},
	},
	{
		ID: "svc-xbox", Cat: "services", Group: "Optional", Title: "Set Xbox services to disabled",
		Desc:    "Xbox Live auth, game save sync and networking services.",
		Benefit: "Fewer background services.",
		Note:    "Breaks Game Pass PC games and Xbox app sign-in. Controllers keep working. Manual only.",
		Impact:  1, Risk: "moderate", Profile: 0,
		Actions: []Action{Svc("XblAuthManager", "disabled"), Svc("XblGameSave", "disabled"), Svc("XboxNetApiSvc", "disabled")},
	},
	{
		ID: "svc-upnp", Cat: "services", Group: "Optional", Title: "Disable network device discovery (SSDP/UPnP)",
		Desc:    "Stops scanning your network for media devices and smart TVs.",
		Benefit: "Less background network chatter.",
		Note:    "Casting to TVs and finding network printers stops working. Manual only.",
		Impact:  1, Risk: "moderate", Profile: 0,
		Actions: []Action{Svc("SSDPSRV", "disabled"), Svc("upnphost", "disabled")},
	},
	{
		ID: "svc-dps", Cat: "services", Group: "Optional", Title: "Disable Diagnostic Policy Service",
		Desc:    "Detects and troubleshoots problems with Windows components.",
		Benefit: "One less always-running service.",
		Note:    "Windows troubleshooters stop working. Manual only.",
		Impact:  1, Risk: "moderate", Profile: 0,
		Actions: []Action{Svc("DPS", "disabled"), Svc("WdiServiceHost", "manual"), Svc("WdiSystemHost", "manual")},
	},

	// ────────────────────────────── BIOS & KERNEL ──────────────────────────────
	{
		ID: "dynamic-tick", Cat: "kernel", Group: "Boot timers", Title: "Disable dynamic tick",
		Desc:    "Stops the kernel skipping timer interrupts to save power.",
		Benefit: "More consistent timer resolution and smoother frame pacing.",
		Impact:  2, Risk: "moderate", Profile: 2, Reboot: true,
		Actions: []Action{BCD("disabledynamictick", "yes")},
	},
	{
		ID: "platform-clock", Cat: "kernel", Group: "Boot timers", Title: "Stop forcing HPET as the system clock",
		Desc:    "Removes the \"useplatformclock\" boot flag so Windows uses the CPU's fast invariant TSC timer.",
		Benefit: "Lower overhead every time a game reads the clock.",
		Note:    "Most PCs don't have this flag set; this just makes sure.",
		Impact:  2, Risk: "safe", Profile: 2, Reboot: true,
		Actions: []Action{BCD("useplatformclock", "")},
	},
	{
		ID: "tsc-sync", Cat: "kernel", Group: "Boot timers", Title: "Enhanced TSC sync",
		Desc:    "Sets the timestamp-counter sync policy to enhanced across all cores.",
		Benefit: "Tighter cross-core timing.",
		Impact:  1, Risk: "moderate", Profile: 2, Reboot: true,
		Actions: []Action{BCD("tscsyncpolicy", "enhanced")},
	},
	{
		ID: "platform-tick", Cat: "kernel", Group: "Boot timers", Title: "Use the platform tick",
		Desc:    "Forces the kernel to use the platform clock as its tick source.",
		Benefit: "Some players report steadier timing; results vary by motherboard.",
		Note:    "Benchmark before and after. Manual only.",
		Impact:  1, Risk: "expert", Profile: 0, Reboot: true,
		Actions: []Action{BCD("useplatformtick", "yes")},
	},
	{
		ID: "mitigations", Cat: "kernel", Group: "Security trade-offs", Title: "Disable CPU Spectre/Meltdown mitigations",
		Desc:    "Turns off Windows' software protections against speculative-execution attacks.",
		Benefit: "Real CPU performance back, mainly on older Intel CPUs (6th–9th gen).",
		Note:    "Security trade-off: only on a trusted gaming PC. Manual only.",
		Impact:  2, Risk: "expert", Profile: 0, Reboot: true,
		Actions: []Action{
			RegD(HKLM, pMemMgmt, "FeatureSettingsOverride", 3),
			RegD(HKLM, pMemMgmt, "FeatureSettingsOverrideMask", 3),
		},
	},
	{
		ID: "hvci", Cat: "kernel", Group: "Security trade-offs", Title: "Turn off Memory Integrity (HVCI)",
		Desc:    "Disables hypervisor-enforced code integrity, one of the core isolation features.",
		Benefit: "Microsoft has confirmed it can cost some FPS in games.",
		Note:    "Security trade-off. Some anti-cheats may ask you to turn it back on. Manual only.",
		Impact:  2, Risk: "expert", Profile: 0, Reboot: true,
		Actions: []Action{RegD(HKLM, pHVCI, "Enabled", 0)},
	},
	{
		ID: "vbs", Cat: "kernel", Group: "Security trade-offs", Title: "Turn off virtualization-based security",
		Desc:    "Disables VBS and stops the Windows hypervisor loading at boot.",
		Benefit: "Removes hypervisor overhead from every system call.",
		Note:    "WSL2, Hyper-V, Windows Sandbox and Android apps stop working. Security trade-off. Manual only.",
		Impact:  2, Risk: "expert", Profile: 0, Reboot: true,
		Actions: []Action{
			RegD(HKLM, pDevGuard, "EnableVirtualizationBasedSecurity", 0),
			BCD("hypervisorlaunchtype", "off"),
		},
	},

	// ────────────────────────────── APPEARANCE (manual only) ──────────────────────────────
	{
		ID: "transparency", Cat: "appearance", Group: "Effects", Title: "Turn off transparency effects",
		Desc:    "Taskbar, Start and windows become solid instead of see-through.",
		Benefit: "Slightly less work for the desktop compositor.",
		Note:    "Changes how Windows looks.",
		Impact:  1, Risk: "safe", Profile: 0,
		Actions: []Action{RegD(HKCU, pPersonalize, "EnableTransparency", 0)},
	},
	{
		ID: "taskbar-anim", Cat: "appearance", Group: "Effects", Title: "Turn off taskbar animations",
		Desc:    "Taskbar buttons stop animating.",
		Benefit: "Snappier taskbar.",
		Note:    "Changes how Windows looks.",
		Impact:  1, Risk: "safe", Profile: 0,
		Actions: []Action{RegD(HKCU, pExplAdv, "TaskbarAnimations", 0)},
	},
	{
		ID: "window-anim", Cat: "appearance", Group: "Effects", Title: "Turn off minimise and maximise animations",
		Desc:    "Windows snap open and closed instantly.",
		Benefit: "Alt-tabbing in and out of Fortnite feels instant.",
		Note:    "Changes how Windows looks.",
		Impact:  1, Risk: "safe", Profile: 0,
		Actions: []Action{RegS(HKCU, `Control Panel\Desktop\WindowMetrics`, "MinAnimate", "0")},
	},
}

// CatalogStats summarises counts for the UI.
func CatalogStats() (tweaks, actions int) {
	for _, t := range Catalog {
		tweaks++
		actions += len(t.Actions)
	}
	return
}
