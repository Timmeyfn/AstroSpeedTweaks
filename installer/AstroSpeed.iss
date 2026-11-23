; AstroSpeed Tweaks installer for Inno Setup (free: https://jrsoftware.org/isdl.php)
; Open this file in Inno Setup, press Build > Compile, and you get AstroSpeed-Setup.exe
; (a normal next-next-finish installer with a Start Menu entry and an uninstaller).

#define AppName "AstroSpeed Tweaks"
#define AppVersion "1.6.0"
#define AppPublisher "AstroSpeed"
#define AppExe "AstroSpeed Tweaks.exe"

[Setup]
AppId={{B6B6F2A1-6C2E-4E7B-9E2A-AST0SPEED1606}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
DefaultDirName={autopf}\AstroSpeed Tweaks
DefaultGroupName=AstroSpeed Tweaks
DisableProgramGroupPage=yes
OutputDir=.
OutputBaseFilename=AstroSpeed-Setup
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
; The app itself asks for administrator when it needs it, so the installer runs as the normal user.
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
UninstallDisplayIcon={app}\{#AppExe}
ArchitecturesInstallIn64BitMode=x64compatible

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Shortcuts:"

[Files]
; Put AstroSpeed Tweaks.exe in the same folder as this script before compiling.
Source: "AstroSpeed Tweaks.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\AstroSpeed Tweaks"; Filename: "{app}\{#AppExe}"
Name: "{group}\Uninstall AstroSpeed Tweaks"; Filename: "{uninstallexe}"
Name: "{autodesktop}\AstroSpeed Tweaks"; Filename: "{app}\{#AppExe}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#AppExe}"; Description: "Open AstroSpeed Tweaks now"; Flags: nowait postinstall skipifsilent
