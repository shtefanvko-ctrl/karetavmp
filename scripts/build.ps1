$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path -Parent $PSScriptRoot
$Dist = Join-Path $RepoRoot "dist"
New-Item -ItemType Directory -Force -Path $Dist | Out-Null

Push-Location $RepoRoot
try {
    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"

    go test ./...
    go build -trimpath -ldflags "-s -w" -o (Join-Path $Dist "KARETA-Node.exe") ./cmd/kareta-node

    $Hash = Get-FileHash (Join-Path $Dist "KARETA-Node.exe") -Algorithm SHA256
    $Hash | Format-List
}
finally {
    Pop-Location
}
