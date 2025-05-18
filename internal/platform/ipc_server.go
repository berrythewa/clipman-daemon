package platform

import (
	"fmt"
	"os"
	"github.com/berrythewa/clipman-daemon/internal/ipc"
)

// StartClipmanIPCServer launches the IPC server in a goroutine.
func StartClipmanIPCServer() {
	go func() {
		err := ipc.ListenAndServe("", handleIPCRequest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "IPC server error: %v\n", err)
		}
	}()
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
