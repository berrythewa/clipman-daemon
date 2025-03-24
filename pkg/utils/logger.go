package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)
var osExit = os.Exit


type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

// Logger implements a structured logger with level filtering
type Logger struct {
	level           LogLevel      // Minimum level for console output
	fileLevel       LogLevel      // Minimum level for file output (can be different)
	logger          *log.Logger   // Logger for stdout
	fileLogger      *log.Logger   // Logger for file output
	logFile         *os.File      // File handle for log file
	logPath         string        // Path to the current log file
	mu              sync.Mutex    // Protects file operations
	fileSize        int64         // Current log file size
	maxSize         int64         // Maximum size before rotation (default 10MB)
	maxFiles        int           // Maximum number of log files to keep (default 5)
	enableConsole   bool          // Whether to output to console
}

// LoggerOptions defines options for configuring the logger
type LoggerOptions struct {
	Level           string    // Log level for console (debug, info, warn, error, fatal)
	FileLevel       string    // Log level for file (defaults to same as Level)
	Output          io.Writer // Output writer (defaults to stdout if nil)
	LogDir          string    // Directory for log files (if empty, file logging is disabled)
	MaxSize         int64     // Maximum size of log file before rotation in bytes (default 10MB)
	MaxFiles        int       // Maximum number of log files to keep (default 5)
	DisableConsole  bool      // If true, disables console output (useful for tests)
}

// NewLogger creates a new logger with the specified options
func NewLogger(options LoggerOptions) *Logger {
	level := parseLogLevel(options.Level)
	fileLevel := level
	if options.FileLevel != "" {
		fileLevel = parseLogLevel(options.FileLevel)
	}
	
	out := options.Output
	if out == nil {
		out = os.Stdout
	}
	
	// Default values
	maxSize := options.MaxSize
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024 // 10MB default
	}
	
	maxFiles := options.MaxFiles
	if maxFiles <= 0 {
		maxFiles = 5 // Default to keeping 5 log files
	}
	
	logger := &Logger{
		level:         level,
		fileLevel:     fileLevel,
		logger:        log.New(out, "", log.Ldate|log.Ltime),
		maxSize:       maxSize,
		maxFiles:      maxFiles,
		enableConsole: !options.DisableConsole,
	}
	
	// Set up file logging if a log directory is provided
	if options.LogDir != "" {
		if err := os.MkdirAll(options.LogDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
		} else {
			// Create a log file with timestamp in the filename for uniqueness
			timestamp := time.Now().Format("20060102")
			logPath := filepath.Join(options.LogDir, fmt.Sprintf("clipman_%s.log", timestamp))
			logger.logPath = logPath
			
			// Try to open the log file
			f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
			} else {
				// Get current file size
				info, err := f.Stat()
				if err == nil {
					logger.fileSize = info.Size()
				}
				
				logger.logFile = f
				logger.fileLogger = log.New(f, "", log.Ldate|log.Ltime)
				
				// Write startup message to log file
				logger.fileLogger.Println("Log file opened, logging started at log level", options.Level)
			}
		}
	}
	
	return logger
}

// Close closes the log file if it's open
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.logFile != nil {
		// Write closing message
		if l.fileLogger != nil {
			l.fileLogger.Println("Log file closed")
			l.logFile.Sync() // Force flush to disk
		}
		return l.logFile.Close()
	}
	return nil
}

// Flush forces any buffered log data to be written to disk
func (l *Logger) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.logFile != nil {
		return l.logFile.Sync()
	}
	return nil
}

// rotateLog rotates the log file if it exceeds the maximum size
func (l *Logger) rotateLog() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.logFile == nil || l.fileSize < l.maxSize {
		return nil
	}
	
	// Log a message about rotation
	rotationMsg := fmt.Sprintf("Log file reached max size (%.2f MB), rotating", float64(l.fileSize)/1024/1024)
	if l.fileLogger != nil {
		l.fileLogger.Println(rotationMsg)
	}
	
	// Close current log file
	l.logFile.Sync() // Force flush
	l.logFile.Close()
	
	// Rename current log file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	newPath := fmt.Sprintf("%s.%s", l.logPath, timestamp)
	err := os.Rename(l.logPath, newPath)
	if err != nil {
		return fmt.Errorf("failed to rotate log file: %v", err)
	}
	
	// Open new log file
	f, err := os.OpenFile(l.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open new log file: %v", err)
	}
	
	// Update logger with new file
	l.logFile = f
	l.fileSize = 0
	l.fileLogger = log.New(f, "", log.Ldate|log.Ltime)
	
	// Log a message in the new file
	l.fileLogger.Println("New log file created after rotation")
	
	// Clean up old log files
	l.cleanOldLogs()
	
	return nil
}

// cleanOldLogs removes old log files, keeping only the most recent maxFiles
func (l *Logger) cleanOldLogs() {
	if l.logPath == "" || l.maxFiles <= 0 {
		return
	}
	
	dir := filepath.Dir(l.logPath)
	base := filepath.Base(l.logPath)
	
	// Find all rotated log files
	pattern := fmt.Sprintf("%s.*", base)
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return
	}
	
	// If we have fewer files than the limit, do nothing
	if len(matches) <= l.maxFiles {
		return
	}
	
	// Sort files by modification time (oldest first)
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	
	files := make([]fileInfo, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		files = append(files, fileInfo{path: match, modTime: info.ModTime()})
	}
	
	// Sort by modification time (oldest first)
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].modTime.After(files[j].modTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
	
	// Delete oldest files that exceed the limit
	for i := 0; i < len(files)-l.maxFiles; i++ {
		os.Remove(files[i].path)
	}
}

func parseLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warn":
		return LogLevelWarn
	case "error":
		return LogLevelError
	case "fatal":
		return LogLevelFatal
	default:
		return LogLevelInfo
	}
}

func (l *Logger) log(level LogLevel, msg string, keyvals ...interface{}) {
	// Format the log message with level prefix and key-value pairs
	prefix := fmt.Sprintf("[%s] ", levelToString(level))
	logMsg := prefix + msg
	
	// Format key-value pairs
	var kvStr string
	if len(keyvals) > 0 {
		kvStr = " |"
		for i := 0; i < len(keyvals); i += 2 {
			if i+1 < len(keyvals) {
				kvStr += fmt.Sprintf(" %v=%v", keyvals[i], keyvals[i+1])
			}
		}
	}
	
	// Combine message and key-value pairs
	fullMsg := []interface{}{logMsg}
	if kvStr != "" {
		fullMsg = append(fullMsg, kvStr)
	}

	// Log to console if level is sufficient
	if l.enableConsole && level >= l.level {
		l.logger.Println(fullMsg...)
	}
	
	// Log to file if enabled and level is sufficient for file logging
	l.mu.Lock()
	if l.fileLogger != nil && l.logFile != nil && level >= l.fileLevel {
		// Write to file logger
		l.fileLogger.Println(fullMsg...)
		
		// Force flush to disk for important messages
		if level >= LogLevelError {
			l.logFile.Sync()
		}
		
		// Update file size and check if rotation is needed
		l.fileSize += int64(len(fmt.Sprint(fullMsg...))) + 1 // +1 for newline
		l.mu.Unlock()
		l.rotateLog() // This will acquire the lock again
	} else {
		l.mu.Unlock()
	}
}

func (l *Logger) Debug(msg string, keyvals ...interface{}) { l.log(LogLevelDebug, msg, keyvals...) }
func (l *Logger) Info(msg string, keyvals ...interface{})  { l.log(LogLevelInfo, msg, keyvals...) }
func (l *Logger) Warn(msg string, keyvals ...interface{})  { l.log(LogLevelWarn, msg, keyvals...) }
func (l *Logger) Error(msg string, keyvals ...interface{}) { l.log(LogLevelError, msg, keyvals...) }
func (l *Logger) Fatal(msg string, keyvals ...interface{}) {
	l.log(LogLevelFatal, msg, keyvals...)
	
	// Make sure log messages are flushed before exiting
	if l.logFile != nil {
		l.logFile.Sync()
		l.logFile.Close()
	}
	
	osExit(1)
}

func levelToString(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}