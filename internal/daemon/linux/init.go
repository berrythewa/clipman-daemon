//go:build linux
// +build linux

package daemon

// init registers the Linux daemonizer implementation
func init() {
	defaultDaemonizer = NewDaemonizer()
} 