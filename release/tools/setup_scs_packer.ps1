$ErrorActionPreference = "Stop"
$Tools = Split-Path -Parent $MyInvocation.MyCommand.Path
$Url = "https://download.eurotrucksimulator2.com/scs_packer_1_55.zip"
$Zip = Join-Path $env:TEMP "scs_packer_1_55.zip"
$Tmp = Join-Path $env:TEMP ("scs_packer_" + [Guid]::NewGuid().ToString("N"))
Write-Host "Downloading official SCS Game Archive Packer (1.55+)…"
Invoke-WebRequest -Uri $Url -OutFile $Zip
New-Item -ItemType Directory -Force -Path $Tmp | Out-Null
Expand-Archive -Path $Zip -DestinationPath $Tmp -Force
$Exe = Get-ChildItem -Path $Tmp -Recurse -Filter "scs_packer*.exe" | Select-Object -First 1
if (-not $Exe) { throw "scs_packer.exe was not found in the official archive." }
Copy-Item $Exe.FullName (Join-Path $Tools "scs_packer.exe") -Force
Write-Host "Installed:" (Join-Path $Tools "scs_packer.exe")
Write-Host "You can now scan HashFS v1/v2 .scs packages in ETS2OMSI Alpha."
Remove-Item $Tmp -Recurse -Force
Remove-Item $Zip -Force
Read-Host "Press Enter to close"
