#ifndef AppVersion
  #define AppVersion "0.1.0"
#endif
[Setup]
AppId={{9D45D077-0787-4CBB-9431-7A1D6221F91B}
AppName=GameMic
AppVersion={#AppVersion}
AppPublisher=qiudeng7
AppPublisherURL=https://github.com/qiudeng7/GameMic
DefaultDirName={autopf}\GameMic
DefaultGroupName=GameMic
DisableProgramGroupPage=yes
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
OutputDir=..\dist
OutputBaseFilename=GameMic-Setup-{#AppVersion}-windows-amd64
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\GameMic.exe
InfoBeforeFile=INSTALL-NOTES.txt
CloseApplications=yes
RestartApplications=no

[Tasks]
Name: desktopicon; Description: "Create a desktop shortcut"; Flags: unchecked
Name: cable; Description: "Install VB-CABLE virtual microphone (required on first use)"; Flags: checkedonce

[Files]
Source: "..\dist\GameMic.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\licenses\*"; DestDir: "{app}\licenses"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "vendor\VBCABLE\*"; DestDir: "{app}\driver"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\GameMic"; Filename: "{app}\GameMic.exe"
Name: "{group}\Uninstall GameMic"; Filename: "{uninstallexe}"
Name: "{autodesktop}\GameMic"; Filename: "{app}\GameMic.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\driver\VBCABLE_Setup_x64.exe"; WorkingDir: "{app}\driver"; StatusMsg: "Click Install Driver in the VB-CABLE window, then restart Windows."; Flags: waituntilterminated; Tasks: cable; Check: NeedsCable
Filename: "{app}\GameMic.exe"; Description: "Open GameMic (restart Windows first if you just installed VB-CABLE)"; Flags: nowait postinstall skipifsilent runasoriginaluser unchecked

[Code]
function NeedsCable: Boolean;
var Code: Integer;
begin
  Result := True;
  if Exec(ExpandConstant('{app}\GameMic.exe'), '--check-cable', '', SW_HIDE, ewWaitUntilTerminated, Code) then
    Result := Code <> 0;
end;
