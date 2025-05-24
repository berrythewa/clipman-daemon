//go:build linux
// +build linux

package platform

import (
	"time"

	"go.uber.org/zap"
)

// MonitorChanges starts monitoring for clipboard changes
func (c *LinuxClipboard) MonitorChanges(handler ClipboardChangeHandler) {
	c.mu.Lock()
	if c.isRunning {
		c.mu.Unlock()
		return
	}
	c.isRunning = true
	c.mu.Unlock()

	go func() {
		interval := c.baseInterval
		for {
			select {
			case <-c.ctx.Done():
				c.logger.Debug("Stopping clipboard monitoring")
				return
			case <-time.After(interval):
				content, err := c.Read()
				if err != nil {
					c.logger.Error("Failed to read clipboard", zap.Error(err))
					continue
				}

				c.mu.Lock()
				if content != nil && string(content.Data) != c.lastContent {
					c.lastContent = string(content.Data)
					c.inactiveStreak = 0
					interval = c.baseInterval
					c.mu.Unlock()
					
					if !c.stealthMode {
						handler(content)
					}
				} else {
					c.inactiveStreak++
					if c.inactiveStreak > 5 {
						interval = c.maxInterval
					}
					c.mu.Unlock()
				}
			}
		}
	}()
} 