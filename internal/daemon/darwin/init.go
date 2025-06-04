//go:build darwin
// +build darwin

package daemon

// init registers the Darwin (macOS) daemonizer implementation
func init() {
	defaultDaemonizer = NewDaemonizer()
} 