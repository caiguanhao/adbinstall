#define AppName "Android Updater"
#define AppNameNoSpace "AndroidUpdater"
#define AppNameShort "adbinstall"
#define AppExe "adbinstall.exe"

[Setup]
AppName={#AppName}
AppVersion=1.0
WizardStyle=modern
DefaultDirName={autopf}\{#AppNameNoSpace}
DefaultGroupName={#AppName}
UninstallDisplayIcon={app}\{#AppExe}
Compression=lzma2/fast
SolidCompression=yes
;Compression=none
OutputDir=.
OutputBaseFilename={#AppNameShort}-{#SetupSetting("AppVersion")}
SetupIconFile=icon.ico

[Run]
Filename: "{app}\{#AppExe}"; Description: "Launch application"; Flags: postinstall nowait skipifsilent

[Tasks]
Name: desktopicon; Description: "Create a &desktop icon"; GroupDescription: "Additional icons:"

[Files]
Source: "{#AppExe}"; DestDir: "{app}"
Source: "lib\*"; DestDir: "{app}\lib"

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExe}"
Name: "{commondesktop}\{#AppName}"; Filename: "{app}\{#AppExe}"; Tasks: desktopicon
