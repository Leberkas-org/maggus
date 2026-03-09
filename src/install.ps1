$scriptpath = $MyInvocation.MyCommand.Path
$dir = Split-Path $scriptpath
Push-Location $dir

Write-Host "Compile to C:\bin\maggus.exe"

go build -o C:\bin\maggus.exe

Pop-Location