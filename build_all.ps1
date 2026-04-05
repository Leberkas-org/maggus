param(
    [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"

$targets = @(
    @{ GOOS = "linux";   GOARCH = "amd64"; Ext = "" },
    @{ GOOS = "linux";   GOARCH = "arm64"; Ext = "" },
    @{ GOOS = "darwin";  GOARCH = "amd64"; Ext = "" },
    @{ GOOS = "darwin";  GOARCH = "arm64"; Ext = "" },
    @{ GOOS = "windows"; GOARCH = "amd64"; Ext = ".exe" }
)

$binDir = Join-Path $PSScriptRoot "bin"
New-Item -ItemType Directory -Force -Path $binDir | Out-Null

Push-Location (Join-Path $PSScriptRoot "src")
try {
    foreach ($t in $targets) {
        $name = "maggus_$($t.GOOS)_$($t.GOARCH)$($t.Ext)"
        $out  = Join-Path $binDir $name
        Write-Host "Building $name..."
        $env:GOOS   = $t.GOOS
        $env:GOARCH = $t.GOARCH
        go build -ldflags "-s -w -X github.com/leberkas-org/maggus/cmd.Version=$Version" -o $out .
        if ($LASTEXITCODE -ne 0) { throw "Build failed for $name" }
    }
    Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
} finally {
    Pop-Location
}

Write-Host ""
Write-Host "Done. Binaries in ./bin:"
Get-ChildItem $binDir | Select-Object Name, @{N="Size";E={"{0:N0} KB" -f ($_.Length / 1KB)}} | Format-Table -AutoSize
