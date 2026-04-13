$scriptpath = $MyInvocation.MyCommand.Path
$dir = Split-Path $scriptpath
Push-Location $dir

try {
    Write-Host -ForegroundColor DarkRed "Stopping all maggus instances..."
    maggus stop --all
    Stop-Process -Name maggus -Force -ErrorAction SilentlyContinue
    while (Get-Process -Name maggus -ErrorAction SilentlyContinue) {
        Start-Sleep -Milliseconds 200
    }
    $buildTime = Get-Date -Format "HHmmss"
    Write-Host -ForegroundColor DarkCyan "Compile to C:\bin\maggus.exe (dev-$buildTime)"

    go build -ldflags "-X github.com/leberkas-org/maggus/cmd.BuildTime=$buildTime" -o C:\bin\maggus.exe
}
finally {
    Pop-Location
}