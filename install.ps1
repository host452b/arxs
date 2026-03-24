$ErrorActionPreference = "Stop"

$repo = "host452b/arxs"
$version = "v1.0.3"
$binary = "arxs-windows-amd64.exe"
$url = "https://github.com/$repo/releases/download/$version/$binary"
$installDir = "$env:LOCALAPPDATA\arxs"
$installPath = "$installDir\arxs.exe"

Write-Host "Downloading arxs $version for Windows..."
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
Invoke-WebRequest -Uri $url -OutFile $installPath

# Add to PATH if not already there
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$currentPath;$installDir", "User")
    $env:Path = "$env:Path;$installDir"
    Write-Host "Added $installDir to PATH"
}

Write-Host "arxs $version installed to $installPath"
& $installPath --version
