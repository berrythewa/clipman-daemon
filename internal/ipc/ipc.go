package ipc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
)

const (
	// Default socket path for Unix systems
	DefaultSocketPath = "/tmp/clipman.sock"
)

// SendRequest connects to the daemon, sends a request, and returns the response.
func SendRequest(socketPath string, req *Request) (*Response, error) {
	if runtime.GOOS == "windows" {
		return nil, errors.New("IPC not implemented for Windows yet")
	}
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	if err := enc.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	var resp Response
	if err := dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &resp, nil
}

// ListenAndServe starts the IPC server and handles requests using the given handler.
func ListenAndServe(socketPath string, handler func(*Request) *Response) error {
	if runtime.GOOS == "windows" {
		return errors.New("IPC server not implemented for Windows yet")
	}
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}
	// Remove any stale socket
	os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}
	defer ln.Close()
	defer os.Remove(socketPath)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue // Accept next connection
		}
		go handleConn(conn, handler)
	}
}

func handleConn(conn net.Conn, handler func(*Request) *Response) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	var req Request
	if err := dec.Decode(&req); err != nil {
		resp := &Response{Status: "error", Message: "invalid request: " + err.Error()}
		enc.Encode(resp)
		return
	}
	resp := handler(&req)
	enc.Encode(resp)
} 