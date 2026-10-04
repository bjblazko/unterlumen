; Unterlumen-Setup.exe — installs Unterlumen for the current user, without
; administrator rights, with a Start menu entry and an uninstaller
; (ADR-0042, stage 1b). Not signed, so SmartScreen asks once.
;
;   ISCC.exe /DAppVersion=0.15.0 /DSourceDir=dist /O=dist packaging\windows\unterlumen.iss
;
; SourceDir holds unterlumen.exe.

#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif
#ifndef SourceDir
  #define SourceDir "."
#endif

[Setup]
AppId={{7C2A4E61-3B8D-4F0A-9E57-1D6B2C8F4A93}
AppName=Unterlumen
AppVersion={#AppVersion}
AppPublisher=Timo Böwing
AppPublisherURL=https://huepattl.de/products/unterlumen
AppSupportURL=https://github.com/bjblazko/unterlumen
DefaultDirName={autopf}\Unterlumen
DisableDirPage=yes
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
WizardStyle=modern
SetupIconFile=unterlumen.ico
UninstallDisplayIcon={app}\unterlumen.ico
UninstallDisplayName=Unterlumen
OutputBaseFilename=Unterlumen-Setup
Compression=lzma2
SolidCompression=yes
; An update closes a running Unterlumen before it replaces it.
CloseApplications=yes

[Files]
Source: "{#SourceDir}\unterlumen.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "unterlumen.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
; The server keeps a console window for its log; it starts minimised, and
; closing the app window stops it.
Name: "{autoprograms}\Unterlumen"; Filename: "{app}\unterlumen.exe"; Parameters: "-desktop"; IconFilename: "{app}\unterlumen.ico"; Flags: runminimized

[Run]
Filename: "{app}\unterlumen.exe"; Parameters: "-desktop"; Description: "Open Unterlumen"; Flags: nowait postinstall skipifsilent runminimized
