package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// LogLevel represents the severity of a log entry
type LogLevel string

const (
	LevelDebug LogLevel = "DEBUG"
	LevelInfo  LogLevel = "INFO"
	LevelWarn  LogLevel = "WARN"
	LevelError LogLevel = "ERROR"
	LevelFatal LogLevel = "FATAL"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	Level       LogLevel               `json:"level"`
	Component   string                 `json:"component"`
	AgentID     string                 `json:"agent_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	Action      string                 `json:"action,omitempty"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	Source      string                 `json:"source,omitempty"`
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	UserID      string                 `json:"user_id"`
	Action      string                 `json:"action"`
	Resource    string                 `json:"resource"`
	ResourceID  string                 `json:"resource_id"`
	Result      string                 `json:"result"`
	Details     map[string]interface{} `json:"details,omitempty"`
	IP          string                 `json:"ip,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
}

// Logger manages structured logging
type Logger struct {
	mu          sync.RWMutex
	redisClient *redis.Client
	logFile     *os.File
	auditFile   *os.File
	logDir      string
	maxSize     int64
	maxAge      time.Duration
	console     bool
}

// NewLogger creates a new logger instance
func NewLogger(redisClient *redis.Client, logDir string, console bool) (*Logger, error) {
	if logDir == "" {
		homeDir, _ := os.UserHomeDir()
		logDir = filepath.Join(homeDir, ".agentainer", "logs")
	}
	
	// Create log directory
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}
	
	// Open log files
	logFile, err := openLogFile(filepath.Join(logDir, "agentainer.log"))
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	
	auditFile, err := openLogFile(filepath.Join(logDir, "audit.log"))
	if err != nil {
		logFile.Close()
		return nil, fmt.Errorf("failed to open audit file: %w", err)
	}
	
	logger := &Logger{
		redisClient: redisClient,
		logFile:     logFile,
		auditFile:   auditFile,
		logDir:      logDir,
		maxSize:     100 * 1024 * 1024, // 100MB
		maxAge:      7 * 24 * time.Hour, // 7 days
		console:     console,
	}
	
	// Start log rotation
	go logger.rotateLoop()
	
	return logger, nil
}

// Close closes the logger
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.logFile != nil {
		l.logFile.Close()
	}
	if l.auditFile != nil {
		l.auditFile.Close()
	}
	
	return nil
}

// Log writes a log entry
func (l *Logger) Log(entry LogEntry) {
	entry.Timestamp = time.Now()
	
	// Write to file
	l.writeToFile(l.logFile, entry)
	
	// Write to Redis for real-time access
	l.writeToRedis("logs", entry)
	
	// Write to console if enabled
	if l.console {
		l.writeToConsole(entry)
	}
}

// Audit writes an audit entry
func (l *Logger) Audit(entry AuditEntry) {
	entry.Timestamp = time.Now()
	
	// Write to file
	l.writeToFile(l.auditFile, entry)
	
	// Write to Redis for real-time access
	l.writeToRedis("audit", entry)
}

// Debug logs a debug message
func (l *Logger) Debug(component, message string, details map[string]interface{}) {
	l.Log(LogEntry{
		Level:     LevelDebug,
		Component: component,
		Message:   message,
		Details:   details,
	})
}

// Info logs an info message
func (l *Logger) Info(component, message string, details map[string]interface{}) {
	l.Log(LogEntry{
		Level:     LevelInfo,
		Component: component,
		Message:   message,
		Details:   details,
	})
}

// Warn logs a warning message
func (l *Logger) Warn(component, message string, details map[string]interface{}) {
	l.Log(LogEntry{
		Level:     LevelWarn,
		Component: component,
		Message:   message,
		Details:   details,
	})
}

// Error logs an error message
func (l *Logger) Error(component, message string, details map[string]interface{}) {
	l.Log(LogEntry{
		Level:     LevelError,
		Component: component,
		Message:   message,
		Details:   details,
	})
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(component, message string, details map[string]interface{}) {
	l.Log(LogEntry{
		Level:     LevelFatal,
		Component: component,
		Message:   message,
		Details:   details,
	})
	os.Exit(1)
}

// GetLogs retrieves logs from Redis
func (l *Logger) GetLogs(ctx context.Context, filter LogFilter) ([]LogEntry, error) {
	key := "logs:entries"
	
	// Get logs from Redis sorted set
	endTime := time.Now()
	startTime := endTime.Add(-filter.Duration)
	
	results, err := l.redisClient.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", startTime.Unix()),
		Max: fmt.Sprintf("%d", endTime.Unix()),
	}).Result()
	
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}
	
	logs := make([]LogEntry, 0, len(results))
	for _, result := range results {
		var entry LogEntry
		if err := json.Unmarshal([]byte(result), &entry); err != nil {
			continue
		}
		
		// Apply filters
		if filter.Level != "" && entry.Level != filter.Level {
			continue
		}
		if filter.Component != "" && entry.Component != filter.Component {
			continue
		}
		if filter.AgentID != "" && entry.AgentID != filter.AgentID {
			continue
		}
		
		logs = append(logs, entry)
	}
	
	// Apply limit
	if filter.Limit > 0 && len(logs) > filter.Limit {
		logs = logs[len(logs)-filter.Limit:]
	}
	
	return logs, nil
}

// GetAuditLogs retrieves audit logs
func (l *Logger) GetAuditLogs(ctx context.Context, filter AuditFilter) ([]AuditEntry, error) {
	key := "audit:entries"
	
	// Get logs from Redis sorted set
	endTime := time.Now()
	startTime := endTime.Add(-filter.Duration)
	
	results, err := l.redisClient.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", startTime.Unix()),
		Max: fmt.Sprintf("%d", endTime.Unix()),
	}).Result()
	
	if err != nil {
		return nil, fmt.Errorf("failed to get audit logs: %w", err)
	}
	
	audits := make([]AuditEntry, 0, len(results))
	for _, result := range results {
		var entry AuditEntry
		if err := json.Unmarshal([]byte(result), &entry); err != nil {
			continue
		}
		
		// Apply filters
		if filter.UserID != "" && entry.UserID != filter.UserID {
			continue
		}
		if filter.Action != "" && entry.Action != filter.Action {
			continue
		}
		if filter.Resource != "" && entry.Resource != filter.Resource {
			continue
		}
		
		audits = append(audits, entry)
	}
	
	// Apply limit
	if filter.Limit > 0 && len(audits) > filter.Limit {
		audits = audits[len(audits)-filter.Limit:]
	}
	
	return audits, nil
}

// LogFilter defines filters for log queries
type LogFilter struct {
	Duration  time.Duration
	Level     LogLevel
	Component string
	AgentID   string
	Limit     int
}

// AuditFilter defines filters for audit log queries
type AuditFilter struct {
	Duration time.Duration
	UserID   string
	Action   string
	Resource string
	Limit    int
}

func (l *Logger) writeToFile(file *os.File, entry interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	
	file.Write(data)
	file.Write([]byte("\n"))
}

func (l *Logger) writeToRedis(prefix string, entry interface{}) {
	ctx := context.Background()
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	
	// Get timestamp
	var timestamp time.Time
	switch e := entry.(type) {
	case LogEntry:
		timestamp = e.Timestamp
	case AuditEntry:
		timestamp = e.Timestamp
	}
	
	// Store in sorted set for time-based queries
	key := fmt.Sprintf("%s:entries", prefix)
	l.redisClient.ZAdd(ctx, key, &redis.Z{
		Score:  float64(timestamp.Unix()),
		Member: string(data),
	})
	
	// Expire old entries (keep 7 days)
	cutoff := time.Now().Add(-7 * 24 * time.Hour).Unix()
	l.redisClient.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", cutoff))
}

func (l *Logger) writeToConsole(entry LogEntry) {
	// Color codes for different levels
	colors := map[LogLevel]string{
		LevelDebug: "\033[36m", // Cyan
		LevelInfo:  "\033[32m", // Green
		LevelWarn:  "\033[33m", // Yellow
		LevelError: "\033[31m", // Red
		LevelFatal: "\033[35m", // Magenta
	}
	
	reset := "\033[0m"
	color := colors[entry.Level]
	
	// Format: [TIMESTAMP] [LEVEL] [COMPONENT] Message
	fmt.Printf("%s[%s] [%s] [%s]%s %s\n",
		color,
		entry.Timestamp.Format("15:04:05"),
		entry.Level,
		entry.Component,
		reset,
		entry.Message,
	)
}

func (l *Logger) rotateLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for range ticker.C {
		l.rotate()
	}
}

func (l *Logger) rotate() {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Check file sizes
	logInfo, _ := l.logFile.Stat()
	auditInfo, _ := l.auditFile.Stat()
	
	// Rotate log file if needed
	if logInfo != nil && logInfo.Size() > l.maxSize {
		l.rotateFile(l.logFile, "agentainer.log")
	}
	
	// Rotate audit file if needed
	if auditInfo != nil && auditInfo.Size() > l.maxSize {
		l.rotateFile(l.auditFile, "audit.log")
	}
	
	// Clean up old files
	l.cleanupOldFiles()
}

func (l *Logger) rotateFile(file *os.File, basename string) {
	// Close current file
	file.Close()
	
	// Rename to timestamped file
	oldPath := filepath.Join(l.logDir, basename)
	newPath := filepath.Join(l.logDir, fmt.Sprintf("%s.%s", basename, time.Now().Format("20060102-150405")))
	os.Rename(oldPath, newPath)
	
	// Open new file
	newFile, err := openLogFile(oldPath)
	if err != nil {
		log.Printf("Failed to open new log file: %v", err)
		return
	}
	
	// Update file reference
	if basename == "agentainer.log" {
		l.logFile = newFile
	} else {
		l.auditFile = newFile
	}
}

func (l *Logger) cleanupOldFiles() {
	files, err := os.ReadDir(l.logDir)
	if err != nil {
		return
	}
	
	cutoff := time.Now().Add(-l.maxAge)
	
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		info, err := file.Info()
		if err != nil {
			continue
		}
		
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(l.logDir, file.Name()))
		}
	}
}

func openLogFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

// TailLogs streams logs as they are written
func (l *Logger) TailLogs(ctx context.Context, filter LogFilter, output io.Writer) error {
	// Subscribe to Redis channel for real-time logs
	pubsub := l.redisClient.Subscribe(ctx, "logs:stream")
	defer pubsub.Close()
	
	ch := pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			var entry LogEntry
			if err := json.Unmarshal([]byte(msg.Payload), &entry); err != nil {
				continue
			}
			
			// Apply filters
			if filter.Level != "" && entry.Level != filter.Level {
				continue
			}
			if filter.Component != "" && entry.Component != filter.Component {
				continue
			}
			if filter.AgentID != "" && entry.AgentID != filter.AgentID {
				continue
			}
			
			// Write to output
			data, _ := json.Marshal(entry)
			output.Write(data)
			output.Write([]byte("\n"))
			
		case <-ctx.Done():
			return nil
		}
	}
}

// Global logger instance
var globalLogger *Logger

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger *Logger) {
	globalLogger = logger
}

// Debug logs a debug message using the global logger
func Debug(component, message string, details map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.Debug(component, message, details)
	}
}

// Info logs an info message using the global logger
func Info(component, message string, details map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.Info(component, message, details)
	}
}

// Warn logs a warning message using the global logger
func Warn(component, message string, details map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.Warn(component, message, details)
	}
}

// Error logs an error message using the global logger
func Error(component, message string, details map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.Error(component, message, details)
	}
}

// AuditLog logs an audit entry using the global logger
func AuditLog(entry AuditEntry) {
	if globalLogger != nil {
		globalLogger.Audit(entry)
	}
}