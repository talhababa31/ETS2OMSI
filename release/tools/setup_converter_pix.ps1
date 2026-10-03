$ErrorActionPreference = "Stop"
$Tools = Split-Path -Parent $MyInvocation.MyCommand.Path
$Url = "https://raw.githubusercontent.com/mwl4/ConverterPIX/master/bin/win_x64/converter_pix.exe"
$Out = Join-Path $Tools "converter_pix.exe"
Write-Host "Downloading ConverterPIX (mwl4) from the official GitHub repository..."
Invoke-WebRequest -Uri $Url -OutFile $Out
$Hash = (Get-FileHash -Algorithm SHA256 $Out).Hash
Write-Host "Installed:" $Out
Write-Host "SHA256:" $Hash
Write-Host "Project / LGPL source: https://github.com/mwl4/ConverterPIX"
Read-Host "Press Enter to close"
