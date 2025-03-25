# Clipman Platform Implementations

This document details how Clipman implements its functionality across different operating systems (Linux, macOS, and Windows), focusing on clipboard monitoring, daemonization, and system integration.

## Architecture Overview

Clipman uses a platform-agnostic core with platform-specific implementations for:

1. **Clipboard Access**: Reading from and writing to the system clipboard
2. **Clipboard Monitoring**: Detecting clipboard changes
3. **Daemonization**: Running as a background process
4. **System Integration**: Integrating with system services

The platform-specific code is organized using Go's build tags to conditionally compile the appropriate implementation.

## Common Interfaces

### Clipboard Interface

```go
type Clipboard interface {
    // Read returns the current clipboard content
    Read() (*types.ClipboardContent, error)
    
    // Write sets the clipboard content
    Write(*types.ClipboardContent) error
    
    // MonitorChanges starts monitoring for clipboard changes
    MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{})
    
    // Close releases any resources
    Close()
}
```

### Daemonizer Interface

```go
type Daemonizer interface {
    // Daemonize forks the current process and runs it in the background
    Daemonize(executable string, args []string, workDir string, dataDir string) (int, error)
    
    // IsRunningAsDaemon returns true if the current process is running as a daemon
    IsRunningAsDaemon() bool
}
```

## Linux Implementation

### Clipboard Monitoring

The Linux implementation uses polling due to limitations in X11 clipboard event notifications:

```go
// LinuxClipboard is the Linux-specific clipboard implementation
type LinuxClipboard struct {
    lastContent []byte
}

// MonitorChanges monitors for clipboard changes
func (c *LinuxClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
    // Polling-based implementation with 500ms interval
    ticker := time.NewTicker(500 * time.Millisecond)
    
    go func() {
        defer ticker.Stop()
        
        for {
            select {
            case <-stopCh:
                return
            case <-ticker.C:
                content, err := c.Read()
                if err != nil {
                    // Skip if content hasn't changed
                    continue
                }
                
                // Send the content to the channel
                contentCh <- content
            }
        }
    }()
}
```

### Daemonization

Linux daemonization uses the traditional Unix approach:

```go
// LinuxDaemonizer implements platform-specific daemonization for Linux
type LinuxDaemonizer struct{}

// Daemonize forks the current process and runs it in the background
func (d *LinuxDaemonizer) Daemonize(executable string, args []string, workDir string, dataDir string) (int, error) {
    // Remove the --detach flag to prevent infinite recursion
    filteredArgs := make([]string, 0, len(args))
    for _, arg := range args {
        if arg != "--detach" {
            filteredArgs = append(filteredArgs, arg)
        }
    }

    // Prepare the command to run
    cmd := exec.Command(executable, filteredArgs...)
    cmd.Dir = workDir

    // Redirect standard file descriptors to /dev/null
    nullDev, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
    if err != nil {
        return 0, fmt.Errorf("failed to open /dev/null: %v", err)
    }
    defer nullDev.Close()
    
    cmd.Stdin = nullDev
    cmd.Stdout = nullDev
    cmd.Stderr = nullDev
    
    // Detach from process group (Unix-specific)
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setsid: true,
    }

    // Create PID file
    // ...
    
    // Start the process
    if err := cmd.Start(); err != nil {
        return 0, fmt.Errorf("failed to start daemon process: %v", err)
    }
    
    return cmd.Process.Pid, nil
}
```

### Linux-Specific Considerations

- **Dependencies**: Uses the `github.com/atotto/clipboard` package for clipboard access
- **X11 Integration**: Works with X11-based desktop environments
- **Polling Interval**: Fixed at 500ms, which balances responsiveness and resource usage
- **Systemd Integration**: Can be installed as a systemd service

## Windows Implementation

### Clipboard Monitoring

Windows implementation uses native clipboard change notifications:

```go
// WindowsClipboard is the Windows-specific clipboard implementation
type WindowsClipboard struct {
    hwnd           windows.Handle
    msgChan        chan uint32
    lastClipFormat uint32
}

// MonitorChanges monitors for clipboard changes
func (c *WindowsClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
    go func() {
        for {
            select {
            case <-stopCh:
                return
            case <-c.msgChan:
                // Clipboard changed, read the new content
                content, err := c.Read()
                if err != nil {
                    if err.Error() == "content unchanged" {
                        continue
                    }
                    continue
                }
                
                // Send the content to the channel
                contentCh <- content
            }
        }
    }()
}
```

### Daemonization

Windows daemonization creates a hidden window:

```go
// WindowsDaemonizer implements platform-specific daemonization for Windows
type WindowsDaemonizer struct{}

// Daemonize forks the current process and runs it in the background
func (d *WindowsDaemonizer) Daemonize(executable string, args []string, workDir string, dataDir string) (int, error) {
    // Remove the --detach flag to prevent infinite recursion
    filteredArgs := make([]string, 0, len(args))
    for _, arg := range args {
        if arg != "--detach" {
            filteredArgs = append(filteredArgs, arg)
        }
    }

    // Prepare the command to run
    cmd := exec.Command(executable, filteredArgs...)
    cmd.Dir = workDir
    
    // Windows-specific: Hide window when running as a daemon
    cmd.SysProcAttr = &syscall.SysProcAttr{
        HideWindow: true,
    }
    
    // Create PID file
    // ...
    
    // Start the process
    if err := cmd.Start(); err != nil {
        return 0, fmt.Errorf("failed to start daemon process: %v", err)
    }
    
    return cmd.Process.Pid, nil
}
```

### Windows-Specific Considerations

- **Windows API**: Uses Windows API directly through the `golang.org/x/sys/windows` package
- **Message Window**: Creates a hidden message window to receive clipboard notifications
- **Event-Based**: Uses the `WM_CLIPBOARDUPDATE` message for efficient notification
- **Format Tracking**: Tracks clipboard sequence number to detect changes

## macOS Implementation

### Clipboard Monitoring

macOS implementation uses NSPasteboard change count for monitoring:

```go
// DarwinClipboard is the macOS-specific clipboard implementation
type DarwinClipboard struct {
    lastChangeCount int
}

// MonitorChanges monitors for clipboard changes
func (c *DarwinClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
    ticker := time.NewTicker(500 * time.Millisecond)
    
    go func() {
        defer ticker.Stop()
        
        for {
            select {
            case <-stopCh:
                return
            case <-ticker.C:
                // Check pasteboard change count
                currentCount := c.getCurrentChangeCount()
                if currentCount != c.lastChangeCount {
                    c.lastChangeCount = currentCount
                    
                    content, err := c.Read()
                    if err != nil {
                        continue
                    }
                    
                    // Send the content to the channel
                    contentCh <- content
                }
            }
        }
    }()
}
```

### Daemonization

macOS daemonization is similar to Linux but with macOS-specific detection:

```go
// DarwinDaemonizer implements platform-specific daemonization for macOS
type DarwinDaemonizer struct{}

// Daemonize forks the current process and runs it in the background
func (d *DarwinDaemonizer) Daemonize(executable string, args []string, workDir string, dataDir string) (int, error) {
    // Similar to Linux implementation but with macOS-specific paths
    // ...
}

// IsRunningAsDaemon returns true if the current process is running as a daemon
func (d *DarwinDaemonizer) IsRunningAsDaemon() bool {
    // Check if we're a session leader (setsid was called)
    pid := os.Getpid()
    pgid, err := syscall.Getpgid(pid)
    if err != nil {
        return false
    }
    
    // If we're not the session leader, we're not a daemon
    if pid != pgid {
        return false
    }
    
    // Check if our ppid is 1 (launchd on macOS)
    ppid := os.Getppid()
    return ppid == 1
}
```

### macOS-Specific Considerations

- **NSPasteboard**: Interacts with the macOS NSPasteboard for clipboard operations
- **Change Count**: Uses change count mechanism for efficient detection
- **launchd Integration**: Can be installed as a launchd service
- **Application Support**: Stores data in macOS-standard Application Support directory

## Configuration and Paths

Each platform uses appropriate system paths for configuration and data:

### Linux
- Config: `~/.config/clipman/config.json`
- Data: `~/.clipman/`

### Windows
- Config: `%APPDATA%\Clipman\config.json`
- Data: `%LOCALAPPDATA%\Clipman\Data`

### macOS
- Config: `~/Library/Application Support/com.berrythewa.clipman/config.json`
- Data: `~/Library/Application Support/Clipman`

## Performance Characteristics

| Platform | Monitoring Method | CPU Usage | Memory Usage | Detection Latency |
|----------|-------------------|-----------|--------------|-------------------|
| Linux    | Polling (500ms)   | Higher    | Lower        | Up to 500ms       |
| Windows  | Event-based       | Lower     | Higher       | Near-instant      |
| macOS    | Change count      | Medium    | Medium       | Up to 500ms       |

## Limitations and Future Improvements

### Linux
- **Wayland Support**: Currently optimized for X11, needs better Wayland integration
- **Event-Based Detection**: Could implement event-based monitoring using X11 events
- **Content Types**: Limited to text content on some distributions

### Windows
- **UWP Integration**: Better integration with UWP clipboard APIs
- **Content Types**: Extend support for more Windows-specific content formats
- **Service Management**: Improve Windows service integration

### macOS
- **Sandboxing**: Better support for sandboxed environments
- **Content Types**: Extend support for more macOS-specific pasteboard types
- **Apple Silicon**: Ensure optimal performance on Apple Silicon 