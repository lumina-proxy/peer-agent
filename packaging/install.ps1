param(
  [string]$Token = $env:LUMINA_TOKEN,
  [string]$Version = $env:LUMINA_VERSION,
  [switch]$Uninstall = ($env:LUMINA_UNINSTALL -eq '1')
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$Base = if ($env:LUMINA_DOWNLOADS) { $env:LUMINA_DOWNLOADS } else { 'https://luminaproxy.com/downloads' }
$Dir = Join-Path $env:LOCALAPPDATA 'LuminaProxy'
$Exe = Join-Path $Dir 'peer-agent.exe'
$Log = Join-Path $Dir 'agent.log'
$RunKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$RunName = 'LuminaProxyPeerAgent'

function Stop-PeerAgent {
  Get-Process -Name 'peer-agent' -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -eq $Exe } |
    Stop-Process -Force -ErrorAction SilentlyContinue
  Start-Sleep -Milliseconds 500
}

if ($Uninstall) {
  Stop-PeerAgent
  Remove-ItemProperty -Path $RunKey -Name $RunName -ErrorAction SilentlyContinue
  if (Test-Path $Dir) { Remove-Item -Recurse -Force $Dir }
  Write-Host 'LuminaProxy peer agent removed.'
  return
}

if (-not $Token) {
  throw 'Missing device token. Get one in Dashboard -> Earn -> Add device, then run: $env:LUMINA_TOKEN="lpk_..."; irm https://luminaproxy.com/downloads/install.ps1 | iex'
}
if ($Token -notmatch '^lpk_[A-Za-z0-9_]+$') {
  throw 'That does not look like a device token (tokens start with lpk_).'
}

function Get-Text([string]$Uri) {
  $Content = (Invoke-WebRequest -UseBasicParsing -Uri $Uri).Content
  if ($Content -is [byte[]]) { $Content = [Text.Encoding]::UTF8.GetString($Content) }
  return [string]$Content
}

if (-not $Version) { $Version = (Get-Text "$Base/VERSION").Trim() }
$Version = $Version.TrimStart('v')
if ($Version -notmatch '^[0-9A-Za-z.\-]+$') { throw 'Could not determine the agent version.' }

$Arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -or $env:PROCESSOR_ARCHITEW6432 -eq 'ARM64') { 'arm64' } else { 'amd64' }
$ZipName = "peer-agent_$($Version)_windows_$Arch.zip"
$Tmp = Join-Path ([IO.Path]::GetTempPath()) ('luminaproxy-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $Tmp | Out-Null

try {
  Write-Host "Downloading peer agent $Version for windows/$Arch ..."
  $Zip = Join-Path $Tmp $ZipName
  Invoke-WebRequest -UseBasicParsing -Uri "$Base/v$Version/$ZipName" -OutFile $Zip
  $Sums = Get-Text "$Base/v$Version/checksums.txt"
  $Expected = $null
  foreach ($Line in ($Sums -split "`r?`n")) {
    $Parts = $Line.Trim() -split '\s+'
    if ($Parts.Count -ge 2 -and $Parts[1] -eq $ZipName) { $Expected = $Parts[0].ToLower() }
  }
  if (-not $Expected) { throw "No checksum published for $ZipName" }
  $Actual = (Get-FileHash -Algorithm SHA256 -Path $Zip).Hash.ToLower()
  if ($Actual -ne $Expected) { throw "Checksum mismatch for $ZipName; aborting." }

  Expand-Archive -Path $Zip -DestinationPath $Tmp -Force
  Stop-PeerAgent
  New-Item -ItemType Directory -Force -Path $Dir | Out-Null
  Copy-Item -Force (Join-Path $Tmp 'peer-agent.exe') $Exe
}
finally {
  Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
}

$Command = '"{0}" --token {1} --i-consent --log-file "{2}"' -f $Exe, $Token, $Log
Set-ItemProperty -Path $RunKey -Name $RunName -Value $Command
Start-Process -FilePath $Exe -ArgumentList @('--token', $Token, '--i-consent', '--log-file', "`"$Log`"") -WindowStyle Hidden

Start-Sleep -Seconds 3
if (Get-Process -Name 'peer-agent' -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $Exe }) {
  Write-Host 'LuminaProxy peer agent is installed and running. It starts automatically when you sign in.'
} else {
  Write-Host "The agent did not stay running. Check the log: $Log"
}
Write-Host "Logs: $Log"
Write-Host 'Your device will show as online in Dashboard -> Earn within a minute.'
Write-Host 'To remove it later: $env:LUMINA_UNINSTALL="1"; irm https://luminaproxy.com/downloads/install.ps1 | iex'
