param(
    [string]$GoExe = 'go',
    [string]$Data = '',
    [string]$Games = '.\.minecraft',
    [switch]$Serve,
    [string]$Addr = '127.0.0.1:17843'
)
$ErrorActionPreference = 'Stop'
Push-Location (Split-Path $PSScriptRoot -Parent)
try {
    $arguments = @('run', './cmd/ohmymc', '--games', $Games)
    if ($Data) { $arguments += @('--data', $Data) }
    if ($Serve) { $arguments += @('--serve', '--addr', $Addr) }
    $ErrorActionPreference = 'Continue'
    & $GoExe @arguments
    if ($LASTEXITCODE -ne 0) { throw 'OhMyMC exited with an error.' }
} finally {
    Pop-Location
}
