# Clipman Windows Uninstallation Script
# This script uninstalls Clipman from Windows systems

# Verify running as administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "This script requires administrator privileges. Please run as administrator."
    exit 1
}

# Set paths
$installDir = "C:\Program Files\Clipman"
$binaryPath = Join-Path $installDir "clipman.exe"
$startMenuDir = Join-Path $env:ProgramData "Microsoft\Windows\Start Menu\Programs\Clipman"

Write-Host "Clipman Uninstaller"
Write-Host "=================="
Write-Host ""

# Remove service if installed
Write-Host "Checking for service installation..."
try {
    $scheduledTask = Get-ScheduledTask -TaskName "Clipman" -ErrorAction SilentlyContinue
    if ($scheduledTask) {
        Write-Host "Found Clipman scheduled task. Removing..."
        Unregister-ScheduledTask -TaskName "Clipman" -Confirm:$false
        Write-Host "Scheduled task removed."
    } else {
        Write-Host "No scheduled task found."
    }
} catch {
    Write-Host "Error checking for scheduled task: $_"
}

# Remove binary and installation directory
if (Test-Path $binaryPath) {
    Write-Host "Removing Clipman binary..."
    
    # First stop any running instances
    try {
        $processes = Get-Process -Name "clipman" -ErrorAction SilentlyContinue
        if ($processes) {
            Write-Host "Stopping running Clipman processes..."
            $processes | Stop-Process -Force
            Start-Sleep -Seconds 1
        }
    } catch {
        Write-Host "Error stopping Clipman processes: $_"
    }
    
    # Remove the binary and directory
    try {
        Remove-Item $binaryPath -Force
        Write-Host "Binary removed."
        
        Remove-Item $installDir -Recurse -Force
        Write-Host "Installation directory removed."
    } catch {
        Write-Host "Error removing binary or installation directory: $_"
    }
} else {
    Write-Host "Clipman binary not found at $binaryPath"
}

# Remove from PATH
$envPath = [Environment]::GetEnvironmentVariable("PATH", "Machine")
if ($envPath.Contains($installDir)) {
    $newPath = ($envPath.Split(';') | Where-Object { $_ -ne $installDir }) -join ';'
    [Environment]::SetEnvironmentVariable("PATH", $newPath, "Machine")
    Write-Host "Removed Clipman from system PATH"
}

# Remove Start Menu shortcut
if (Test-Path $startMenuDir) {
    try {
        Remove-Item $startMenuDir -Recurse -Force
        Write-Host "Removed Start Menu shortcuts"
    } catch {
        Write-Host "Error removing Start Menu shortcuts: $_"
    }
}

# Ask about removing configuration and data
$removeData = Read-Host "Do you want to remove all Clipman data and configuration? (y/N)"
if ($removeData -eq "y" -or $removeData -eq "Y") {
    Write-Host "Removing Clipman data and configuration..."
    
    # Remove user data
    $dataDir = Join-Path $env:USERPROFILE ".clipman"
    if (Test-Path $dataDir) {
        try {
            Remove-Item $dataDir -Recurse -Force
            Write-Host "Data directory removed."
        } catch {
            Write-Host "Error removing data directory: $_"
        }
    }
    
    # Remove configuration
    $configDir = Join-Path ([Environment]::GetFolderPath("ApplicationData")) "clipman"
    if (Test-Path $configDir) {
        try {
            Remove-Item $configDir -Recurse -Force
            Write-Host "Configuration directory removed."
        } catch {
            Write-Host "Error removing configuration directory: $_"
        }
    }
} else {
    Write-Host "Keeping user data and configuration."
    Write-Host "Data directory: $env:USERPROFILE\.clipman"
    Write-Host "To manually remove, run: Remove-Item -Recurse -Force $env:USERPROFILE\.clipman"
}

Write-Host ""
Write-Host "Clipman has been uninstalled from your system." 