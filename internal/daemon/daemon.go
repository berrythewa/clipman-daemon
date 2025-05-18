package daemon

import (
	"fmt"
	"os"
	"github.com/berrythewa/clipman-daemon/internal/platform"
	"github.com/berrythewa/clipman-daemon/internal/ipc"
)

// Start launches the daemon process and IPC server.
func Start() error {
	daemonizer := platform.GetPlatformDaemonizer()
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	workDir, _ := os.Getwd()
	dataDir := os.Getenv("CLIPMAN_DATA_DIR")
	if dataDir == "" {
		dataDir = fmt.Sprintf("%s/.local/share/clipman", os.Getenv("HOME"))
	}
	pid, err := daemonizer.Daemonize(executable, []string{}, workDir, dataDir)
	if err != nil {
		return fmt.Errorf("failed to daemonize: %w", err)
	}
	fmt.Printf("Clipman daemon started with PID %d\n", pid)

	// Start the IPC server in this process
	go func() {
		err := ipc.ListenAndServe("", handleIPCRequest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "IPC server error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the daemon process using the PID file.
func Stop() error {
	dataDir := os.Getenv("CLIPMAN_DATA_DIR")
	if dataDir == "" {
		dataDir = fmt.Sprintf("%s/.local/share/clipman", os.Getenv("HOME"))
	}
	pidFile := fmt.Sprintf("%s/run/clipman.pid", dataDir)
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("failed to read PID file: %w", err)
	}
	var pid int
	_, err = fmt.Sscanf(string(pidBytes), "%d", &pid)
	if err != nil || pid <= 0 {
		return fmt.Errorf("invalid PID in file: %s", string(pidBytes))
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}
	if err := proc.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}
	os.Remove(pidFile)
	fmt.Println("Clipman daemon stopped.")
	return nil
}

// Status checks if the daemon is running.
func Status() error {
	dataDir := os.Getenv("CLIPMAN_DATA_DIR")
	if dataDir == "" {
		dataDir = fmt.Sprintf("%s/.local/share/clipman", os.Getenv("HOME"))
	}
	pidFile := fmt.Sprintf("%s/run/clipman.pid", dataDir)
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Println("Clipman daemon is not running (no PID file found).")
		return nil
	}
	var pid int
	_, err = fmt.Sscanf(string(pidBytes), "%d", &pid)
	if err != nil || pid <= 0 {
		fmt.Println("Clipman daemon is not running (invalid PID file).")
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("Clipman daemon is not running (process not found).")
		return nil
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		fmt.Println("Clipman daemon is not running (process not alive).")
		return nil
	}
	fmt.Printf("Clipman daemon is running with PID %d.\n", pid)
	return nil
}

// handleIPCRequest processes incoming IPC requests from the CLI.
func handleIPCRequest(req *ipc.Request) *ipc.Response {
	switch req.Command {
	case "history":
		// TODO: Implement actual history retrieval
		return &ipc.Response{Status: "ok", Data: "history not implemented"}
	case "flush":
		// TODO: Implement actual flush
		return &ipc.Response{Status: "ok", Message: "Flush not implemented"}
	default:
		return &ipc.Response{Status: "error", Message: "Unknown command"}
	}
} 