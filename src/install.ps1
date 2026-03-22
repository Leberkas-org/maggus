$scriptpath = $MyInvocation.MyCommand.Path
$dir = Split-Path $scriptpath
Push-Location $dir

try {
    $buildTime = Get-Date -Format "HHmmss"
    Write-Host "Compile to C:\bin\maggus.exe (dev-$buildTime)"

    go build -ldflags "-X github.com/leberkas-org/maggus/cmd.BuildTime=$buildTime" -o C:\bin\maggus.exe
}
finally {
    Pop-Location
}