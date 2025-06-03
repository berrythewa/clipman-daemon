# Clipman Windows Installation Script
# This script installs Clipman on Windows systems

# Verify running as administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "This script requires administrator privileges. Please run as administrator."
    exit 1
}

# Set installation parameters
$installDir = "C:\Program Files\Clipman"
$binaryName = "clipman.exe"
$binaryPath = Join-Path $installDir $binaryName
$startMenuDir = Join-Path $env:ProgramData "Microsoft\Windows\Start Menu\Programs\Clipman"

Write-Host "Clipman Installer"
Write-Host "================="
Write-Host ""

# Create installation directory if it doesn't exist
if (!(Test-Path $installDir)) {
    Write-Host "Creating installation directory at $installDir..."
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

# Check if binary exists locally
$localBinaryPath = $null

if (Test-Path "bin\clipman.exe") {
    $localBinaryPath = "bin\clipman.exe"
    Write-Host "Using local binary..."
} elseif (Test-Path "release\clipman-windows-amd64.exe") {
    $localBinaryPath = "release\clipman-windows-amd64.exe"
    Write-Host "Using prebuilt release binary..."
} else {
    # Download from GitHub releases
    Write-Host "Downloading Clipman from GitHub..."
    
    # TODO: Replace with actual release URL when available
    $downloadUrl = "https://github.com/berrythewa/clipman-daemon/releases/latest/download/clipman-windows-amd64.exe"
    $tempPath = Join-Path $env:TEMP "clipman.exe"
    
    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $tempPath
        $localBinaryPath = $tempPath
    } catch {
        Write-Host "Failed to download Clipman: $_"
        exit 1
    }
}

# Copy binary to installation directory
try {
    Copy-Item $localBinaryPath -Destination $binaryPath -Force
    Write-Host "Installed Clipman to $binaryPath"
} catch {
    Write-Host "Failed to copy binary to installation directory: $_"
    exit 1
}

# Clean up temp file if downloaded
if ($localBinaryPath -eq (Join-Path $env:TEMP "clipman.exe")) {
    Remove-Item $localBinaryPath -Force
}

# Add to PATH
$envPath = [Environment]::GetEnvironmentVariable("PATH", "Machine")
if (!$envPath.Contains($installDir)) {
    [Environment]::SetEnvironmentVariable("PATH", "$envPath;$installDir", "Machine")
    Write-Host "Added Clipman to system PATH"
}

# Create Start Menu shortcut
if (!(Test-Path $startMenuDir)) {
    New-Item -ItemType Directory -Path $startMenuDir -Force | Out-Null
}

$shortcutPath = Join-Path $startMenuDir "Clipman.lnk"
$shell = New-Object -ComObject WScript.Shell
$shortcut = $shell.CreateShortcut($shortcutPath)
$shortcut.TargetPath = $binaryPath
$shortcut.Description = "Clipman Clipboard Manager"
$shortcut.WorkingDirectory = $installDir
$shortcut.Save()
Write-Host "Created Start Menu shortcut"

# Ask about service installation
$installService = Read-Host "Do you want to install Clipman as a service to start automatically? (y/N)"
if ($installService -eq "y" -or $installService -eq "Y") {
    Write-Host "Installing Clipman as a service..."
    Start-Process -FilePath $binaryPath -ArgumentList "service", "install", "--start" -Wait
    Write-Host "Service installation complete."
} else {
    Write-Host "Skipping service installation."
    Write-Host "To start Clipman manually, run: clipman"
    Write-Host "To install as a service later, run: clipman service install"
}

Write-Host ""
Write-Host "Installation complete! Enjoy using Clipman."
Write-Host "For documentation, visit: https://github.com/berrythewa/clipman-daemon" 