param(
    [string]$GoExe = 'go',
    [switch]$SkipChecks
)
$ErrorActionPreference = 'Stop'
$project = Split-Path $PSScriptRoot -Parent
Push-Location $project
try {
    # Windows PowerShell may turn native stderr progress into ErrorRecords.
    # External commands are checked by exit code, not by whether they log stderr.
    $ErrorActionPreference = 'Continue'
    if (-not $SkipChecks) {
        & $GoExe mod verify
        if ($LASTEXITCODE -ne 0) { throw 'Dependency verification failed.' }
        & $GoExe test ./... -count=1 -timeout=90s
        if ($LASTEXITCODE -ne 0) { throw 'Tests failed.' }
        & $GoExe vet ./...
        if ($LASTEXITCODE -ne 0) { throw 'go vet failed.' }
    }
    $ErrorActionPreference = 'Stop'
    New-Item -ItemType Directory -Force '.\dist' | Out-Null
    $ErrorActionPreference = 'Continue'
    & $GoExe build -trimpath -ldflags '-s -w -H=windowsgui' -o '.\dist\OhMyMC.exe' ./cmd/ohmymc
    if ($LASTEXITCODE -ne 0) { throw 'Build failed.' }
    $ErrorActionPreference = 'Stop'
    Copy-Item '.\README.md', '.\AGENTS.md', '.\LICENSE', '.\THIRD_PARTY_NOTICES.md' '.\dist' -Force
    Copy-Item '.\docs', '.\licenses' '.\dist' -Recurse -Force
    Write-Host 'Built dist\OhMyMC.exe (Windows desktop, embedded WebUI and plugins).'
} finally {
    Pop-Location
}
