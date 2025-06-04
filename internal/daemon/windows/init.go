//go:build windows
// +build windows

package daemon

import (
	parentDaemon "github.com/berrythewa/clipman-daemon/internal/daemon"
)

// init registers the Windows daemonizer implementation
func init() {
	defaultDaemonizer = NewDaemonizer()
} 