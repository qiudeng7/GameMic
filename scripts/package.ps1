$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
$version = (Get-Content VERSION -Raw).Trim()
if ($version -notmatch '^\d+\.\d+\.\d+$') { throw 'Invalid VERSION' }
New-Item -ItemType Directory -Force dist, dist/licenses, build/vendor | Out-Null

# Build dependencies are pinned in go.mod; this records the complete checksum set.
go mod tidy
if ($LASTEXITCODE -ne 0) { throw 'go mod tidy failed' }
gofmt -w cmd internal
if ($LASTEXITCODE -ne 0) { throw 'gofmt failed' }
# Export the exact normalized sources/checksums for build provenance.
$sourceMap = @{}
Get-ChildItem cmd,internal -Filter *.go -Recurse | ForEach-Object {
  $relative = [IO.Path]::GetRelativePath($PWD.Path, $_.FullName).Replace('\','/')
  $sourceMap[$relative] = [IO.File]::ReadAllText($_.FullName)
}
$sourceMap['go.mod'] = [IO.File]::ReadAllText((Join-Path $PWD 'go.mod'))
$sourceMap['go.sum'] = [IO.File]::ReadAllText((Join-Path $PWD 'go.sum'))
foreach ($path in $sourceMap.Keys) {
  Write-Output ('GAMEMIC_SOURCE_B64:' + $path + ':' + [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($sourceMap[$path])))
}
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Module verification failed' }
go test ./internal/...
if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }

Push-Location build
try {
  windres -i resources.rc -O coff -o ../cmd/gamemic/resource_windows_amd64.syso
  if ($LASTEXITCODE -ne 0) { throw 'Resource compiler failed' }
} finally { Pop-Location }
go build -trimpath -ldflags "-s -w -H=windowsgui -X main.version=$version -extldflags=-static" -o dist/GameMic.exe ./cmd/gamemic
if ($LASTEXITCODE -ne 0) { throw 'Windows build failed' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }

# GUI creation/message-loop smoke check. No physical microphone is needed.
$smoke = Start-Process -FilePath (Resolve-Path dist/GameMic.exe) -ArgumentList '--smoke-test' -PassThru
if (-not $smoke.WaitForExit(30000)) { $smoke.Kill(); throw 'GUI smoke check timed out' }
if ($smoke.ExitCode -ne 0) { throw "GUI smoke check failed: $($smoke.ExitCode)" }

# Fetch only the official base cable package, leaving all vendor files intact.
$url = 'https://download.vb-audio.com/Download_CABLE/VBCABLE_Driver_Pack45.zip'
$zip = Join-Path $PWD 'build/vendor/VBCABLE_Driver_Pack45.zip'
Invoke-WebRequest -Uri $url -OutFile $zip
$hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLowerInvariant()
$pinPath = Join-Path $PWD 'build/vbcable.sha256'
if (Test-Path $pinPath) {
  $expected = (Get-Content $pinPath -Raw).Trim().Split(' ')[0].ToLowerInvariant()
  if ($hash -ne $expected) { throw "VB-CABLE archive changed: expected $expected, got $hash" }
}
Write-Output "VB-CABLE SHA256: $hash"
Expand-Archive -Path $zip -DestinationPath build/vendor/VBCABLE -Force
$installer = Get-Item build/vendor/VBCABLE/VBCABLE_Setup_x64.exe
$sig = Get-AuthenticodeSignature $installer
if ($sig.Status -ne 'Valid') { throw "VB-CABLE installer signature is not valid: $($sig.Status)" }
if ($sig.SignerCertificate.Subject -notmatch 'Vincent|Burel|VB.?Audio') { throw "Unexpected VB-CABLE signer: $($sig.SignerCertificate.Subject)" }
Write-Output "VB-CABLE signer: $($sig.SignerCertificate.Subject)"
"Source: $url`nSHA256: $hash`nSigner: $($sig.SignerCertificate.Subject)" | Set-Content dist/licenses/VBCABLE-PROVENANCE.txt

Copy-Item LICENSE dist/licenses/GameMic-LICENSE.txt
Copy-Item THIRD-PARTY-NOTICES.md dist/licenses/
$goRoot = go env GOROOT
Copy-Item (Join-Path $goRoot 'LICENSE') dist/licenses/Go-LICENSE.txt
$modules = go list -m -json all | Out-String
$modules | Set-Content dist/licenses/go-modules.json
# Collect licenses from module directories for redistribution.
$moduleDirs = go list -m -f '{{if .Dir}}{{.Path}}|{{.Dir}}{{end}}' all
foreach ($entry in $moduleDirs) {
  if (-not $entry -or $entry.StartsWith('github.com/qiudeng7/GameMic|')) { continue }
  $parts = $entry.Split('|', 2)
  $safeName = $parts[0] -replace '[/\\]', '_'
  foreach ($file in Get-ChildItem -Path $parts[1] -File | Where-Object { $_.Name -match '^(LICENSE|COPYING|UNLICENSE|NOTICE)' }) {
    Copy-Item $file.FullName "dist/licenses/$safeName-$($file.Name)"
  }
}
Copy-Item go.mod,go.sum dist/
Copy-Item README.md dist/

$iscc = Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6/ISCC.exe'
if (-not (Test-Path $iscc)) { throw 'Install Inno Setup 6 before packaging' }
& $iscc "/DAppVersion=$version" build/installer.iss
if ($LASTEXITCODE -ne 0) { throw 'Installer build failed' }

# Portable package is for PCs with VB-CABLE already installed.
Compress-Archive -Path dist/GameMic.exe,dist/licenses,dist/README.md -DestinationPath "dist/GameMic-$version-windows-amd64-portable.zip" -Force
$files = Get-ChildItem dist -File | Where-Object { $_.Extension -in '.exe','.zip' }
$lines = foreach ($file in $files) { "$((Get-FileHash $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant())  $($file.Name)" }
$lines | Set-Content -Encoding ascii dist/SHA256SUMS.txt
Get-Content go.mod
Get-Content go.sum
