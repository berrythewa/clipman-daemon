package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
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

type Logger struct {
	level  LogLevel
	logger *log.Logger
}

func NewLogger(level string, out io.Writer) *Logger {
	if out == nil {
		out = os.Stdout
	}
	return &Logger{
		level:  parseLogLevel(level),
		logger: log.New(out, "", log.Ldate|log.Ltime|log.Lshortfile),
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
	if level < l.level {
		return
	}

	args := []interface{}{levelToString(level), msg}
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			args = append(args, fmt.Sprintf("%v=%v", keyvals[i], keyvals[i+1]))
		}
	}

	l.logger.Println(args...)
}

func (l *Logger) Debug(msg string, keyvals ...interface{}) { l.log(LogLevelDebug, msg, keyvals...) }
func (l *Logger) Info(msg string, keyvals ...interface{})  { l.log(LogLevelInfo, msg, keyvals...) }
func (l *Logger) Warn(msg string, keyvals ...interface{})  { l.log(LogLevelWarn, msg, keyvals...) }
func (l *Logger) Error(msg string, keyvals ...interface{}) { l.log(LogLevelError, msg, keyvals...) }
func (l *Logger) Fatal(msg string, keyvals ...interface{}) {
	fmt.Println("Fatal method called") // Added logging
	l.log(LogLevelFatal, msg, keyvals...)
	fmt.Println("About to call os.Exit(1)") // Added logging
	osExit(1) // Use the variable instead of directly calling os.Exit
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