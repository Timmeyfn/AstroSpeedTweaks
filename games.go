package main

import "strings"

// GameDef describes a supported game and where to look for it. Every game gets the same
// five process-level tweaks (GPU choice, network tagging, CPU and disk priority, fullscreen
// optimizations). None of them touch the game's own files, settings or anti-cheat.
type GameDef struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Studio   string   `json:"studio"`
	Exes     []string `json:"exes"`  // process file names
	Color    string   `json:"color"` // accent for the fallback tile when no icon can be read
	SteamDir []string `json:"-"`     // folder names under steamapps\common
	EpicApp  string   `json:"-"`     // Epic Games Launcher AppName
	Riot     string   `json:"-"`     // Riot product id, e.g. valorant.live
	Uninst   []string `json:"-"`     // Windows "Apps" display names
	Paths    []string `json:"-"`     // extra folders to look in (%VARS%, ?: = every drive, * = any folder)
	Rel      []string `json:"-"`     // where the exe usually sits inside the install folder
	Note     string   `json:"note,omitempty"`
	Image    string   `json:"image,omitempty"`  // picture URL (games added from a GitHub pack)
	Online   bool     `json:"online,omitempty"` // shown on the game page
}

var Games = []GameDef{
	{ID: "fortnite", Name: "Fortnite", Studio: "Epic Games", Exes: []string{"FortniteClient-Win64-Shipping.exe"}, Color: "#7B5CFF"},
	{ID: "valorant", Name: "VALORANT", Studio: "Riot Games", Exes: []string{"VALORANT-Win64-Shipping.exe"}, Color: "#FF4655",
		Riot: "valorant.live", Uninst: []string{"VALORANT"}, Paths: []string{`?:\Riot Games\VALORANT\live`},
		Rel: []string{`ShooterGame\Binaries\Win64\VALORANT-Win64-Shipping.exe`}},
	{ID: "cs2", Name: "Counter-Strike 2", Studio: "Valve", Exes: []string{"cs2.exe"}, Color: "#E8A33D",
		SteamDir: []string{"Counter-Strike Global Offensive"}, Rel: []string{`game\bin\win64\cs2.exe`}},
	{ID: "apex", Name: "Apex Legends", Studio: "Respawn", Exes: []string{"r5apex_dx12.exe", "r5apex.exe"}, Color: "#DA292A",
		SteamDir: []string{"Apex Legends"}, Uninst: []string{"Apex Legends"}, Paths: []string{`%ProgramFiles%\EA Games\Apex`, `?:\EA Games\Apex`},
		Rel: []string{`r5apex_dx12.exe`, `r5apex.exe`}},
	{ID: "rust", Name: "Rust", Studio: "Facepunch", Exes: []string{"RustClient.exe"}, Color: "#CD412B",
		SteamDir: []string{"Rust"}, Rel: []string{`RustClient.exe`},
		Note: "Rust uses a lot of RAM. On 16 GB PCs, leave memory compression on (CPU section)."},
	{ID: "sot", Name: "Sea of Thieves", Studio: "Rare", Exes: []string{"SoTGame.exe"}, Color: "#1F9E8E",
		SteamDir: []string{"Sea of Thieves"}, Rel: []string{`Athena\Binaries\Win64\SoTGame.exe`},
		Note: "Works with the Steam version. The Microsoft Store version can't be changed by any app."},
	{ID: "roblox", Name: "Roblox", Studio: "Roblox", Exes: []string{"RobloxPlayerBeta.exe"}, Color: "#4A5568",
		Paths: []string{`%LOCALAPPDATA%\Roblox\Versions\*`, `%ProgramFiles(x86)%\Roblox\Versions\*`, `%ProgramFiles%\Roblox\Versions\*`},
		Rel:   []string{`RobloxPlayerBeta.exe`},
		Note:  "Roblox moves to a new folder every update. If a switch shows it changed back, turn it on again."},
	{ID: "overwatch", Name: "Overwatch 2", Studio: "Blizzard", Exes: []string{"Overwatch.exe"}, Color: "#F99E1A",
		SteamDir: []string{"Overwatch"}, Uninst: []string{"Overwatch"}, Paths: []string{`%ProgramFiles(x86)%\Overwatch`, `?:\Overwatch`},
		Rel: []string{`_retail_\Overwatch.exe`}},
	{ID: "rivals", Name: "Marvel Rivals", Studio: "NetEase", Exes: []string{"Marvel-Win64-Shipping.exe"}, Color: "#E62429",
		SteamDir: []string{"MarvelRivals"}, Uninst: []string{"Marvel Rivals"}, Rel: []string{`MarvelGame\Marvel\Binaries\Win64\Marvel-Win64-Shipping.exe`}},
	{ID: "rocketleague", Name: "Rocket League", Studio: "Psyonix", Exes: []string{"RocketLeague.exe"}, Color: "#0C7BF2",
		SteamDir: []string{"rocketleague"}, EpicApp: "Sugar", Rel: []string{`Binaries\Win64\RocketLeague.exe`}},
	{ID: "lol", Name: "League of Legends", Studio: "Riot Games", Exes: []string{"League of Legends.exe"}, Color: "#C8AA6E",
		Riot: "league_of_legends.live", Uninst: []string{"League of Legends"}, Paths: []string{`?:\Riot Games\League of Legends`},
		Rel: []string{`Game\League of Legends.exe`}},
	{ID: "gta", Name: "Grand Theft Auto V", Studio: "Rockstar Games", Exes: []string{"GTA5_Enhanced.exe", "GTA5.exe"}, Color: "#4E9F3D",
		SteamDir: []string{"Grand Theft Auto V Enhanced", "Grand Theft Auto V"}, Uninst: []string{"Grand Theft Auto V"},
		Paths: []string{`%ProgramFiles%\Rockstar Games\Grand Theft Auto V`, `%ProgramFiles%\Rockstar Games\Grand Theft Auto V Enhanced`},
		Rel:   []string{`GTA5_Enhanced.exe`, `GTA5.exe`}},
}

func gameByID(id string) (GameDef, bool) {
	for _, g := range Games {
		if g.ID == id {
			return g, true
		}
	}
	return GameDef{}, false
}

func init() {
	for _, g := range Games {
		if g.ID == "fortnite" {
			continue // Fortnite keeps its hand-written tweaks in catalog.go
		}
		c, tw := gameTweaks(g)
		Categories = append(Categories, c)
		Catalog = append(Catalog, tw...)
	}
}

// gameTweaks builds the section and the five standard tweaks for one game.
func gameTweaks(g GameDef) (Category, []Tweak) {
	cat := "game-" + g.ID
	when := []string{"game:" + g.ID}
	c := Category{ID: cat, Title: g.Name, Subtitle: "Priority, GPU choice, fullscreen and network marking for " + g.Name + " only. Its in-game settings and files are never opened.", Icon: "gamepad", Game: g.ID}
	var ifeoCPU, ifeoIO, qos []Action
	for _, exe := range g.Exes {
		p := pIFEO + `\` + exe + `\PerfOptions`
		ifeoCPU = append(ifeoCPU, RegD(HKLM, p, "CpuPriorityClass", 3))
		ifeoIO = append(ifeoIO, RegD(HKLM, p, "IoPriority", 3))
		qos = append(qos, Custom("qos_policy", "AstroSpeed "+g.Name+" "+strings.TrimSuffix(exe, ".exe")+"|"+exe+"|46"))
	}
	qos = append([]Action{RegS(HKLM, pTcpQoS, "Do not use NLA", "1")}, qos...)
	return c, []Tweak{
		{ID: g.ID + "-gpu", Cat: cat, Group: "Process", Title: "Always use your high-performance GPU for " + g.Name,
			Desc:    "Tells Windows to run " + g.Name + " on your dedicated graphics card, never on integrated graphics.",
			Benefit: "Fixes laptops and some desktops silently running the game on the slow GPU.",
			Impact:  3, Risk: "safe", Profile: 1, When: when, Actions: []Action{Dyn("gpupref", g.ID)}},
		{ID: g.ID + "-priority", Cat: cat, Group: "Process", Title: "Run " + g.Name + " at high CPU priority",
			Desc:    "Every time " + g.Name + " starts, Windows gives it High priority instead of Normal, so background apps wait their turn.",
			Benefit: "Fewer frame-time spikes when something in the background wakes up.",
			Impact:  2, Risk: "safe", Profile: 2, When: when, Actions: ifeoCPU},
		{ID: g.ID + "-io", Cat: cat, Group: "Process", Title: "Give " + g.Name + " high disk priority",
			Desc:    "The game's file reads jump ahead of background disk activity.",
			Benefit: "Smoother loading of new areas and fewer streaming hitches.",
			Impact:  1, Risk: "safe", Profile: 2, When: when, Actions: ifeoIO},
		{ID: g.ID + "-fso", Cat: cat, Group: "Process", Title: "Disable fullscreen optimizations for " + g.Name,
			Desc:    "Same as ticking \"Disable fullscreen optimizations\" in the game's exe properties. Only affects this game.",
			Benefit: "True exclusive fullscreen, which some players find lower latency.",
			Note:    "If you play in borderless or windowed mode this makes no difference.",
			Impact:  1, Risk: "safe", Profile: 2, When: when, Actions: []Action{Dyn("fso", g.ID)}},
		{ID: g.ID + "-qos", Cat: cat, Group: "Network", Title: "Mark " + g.Name + " traffic as high priority (DSCP 46)",
			Desc:    "Adds a Windows QoS policy that tags the game's packets as real-time traffic.",
			Benefit: "Routers that honour QoS tags send game packets first when someone else is streaming or downloading.",
			Note:    "Many home routers ignore DSCP tags. Harmless if yours does.",
			Impact:  1, Risk: "safe", Profile: 1, Reboot: true, When: when, Actions: qos},
	}
}
