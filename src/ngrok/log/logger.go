package log

import (
	"fmt"
	"io"
	stdlog "log"
	"os"
	"sync"
)

type level int

const (
	levelDebug level = iota
	levelInfo
	levelWarning
	levelError
	levelDisabled
)

var root = &rootLogger{level: levelDisabled, logger: stdlog.New(io.Discard, "", stdlog.LstdFlags)}

type rootLogger struct {
	sync.Mutex
	level  level
	logger *stdlog.Logger
}

func LogTo(target string, levelName string) {
	root.Lock()
	defer root.Unlock()

	root.level = parseLevel(levelName)
	switch target {
	case "stdout":
		root.logger = stdlog.New(os.Stdout, "", stdlog.LstdFlags)
	case "none":
		root.level = levelDisabled
		root.logger = stdlog.New(io.Discard, "", stdlog.LstdFlags)
	default:
		file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			root.logger = stdlog.New(os.Stderr, "", stdlog.LstdFlags)
			root.logLocked(levelError, "ERROR", "Failed to open log file %s: %v", target, err)
			return
		}
		root.logger = stdlog.New(file, "", stdlog.LstdFlags)
	}
}

func parseLevel(levelName string) level {
	switch levelName {
	case "FINEST", "FINE", "DEBUG", "TRACE":
		return levelDebug
	case "INFO":
		return levelInfo
	case "WARNING":
		return levelWarning
	case "ERROR", "CRITICAL":
		return levelError
	default:
		return levelInfo
	}
}

func (r *rootLogger) log(level level, label string, format string, args ...interface{}) error {
	r.Lock()
	defer r.Unlock()
	return r.logLocked(level, label, format, args...)
}

func (r *rootLogger) logLocked(level level, label string, format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	if r.level == levelDisabled || level < r.level {
		return err
	}
	r.logger.Printf("%s %s", label, err.Error())
	return err
}

type Logger interface {
	AddLogPrefix(string)
	ClearLogPrefixes()
	Debug(string, ...interface{})
	Info(string, ...interface{})
	Warn(string, ...interface{}) error
	Error(string, ...interface{}) error
}

type PrefixLogger struct {
	prefix string
}

func NewPrefixLogger(prefixes ...string) Logger {
	logger := &PrefixLogger{}

	for _, p := range prefixes {
		logger.AddLogPrefix(p)
	}

	return logger
}

func (pl *PrefixLogger) pfx(fmtstr string) string {
	return fmt.Sprintf("%s %s", pl.prefix, fmtstr)
}

func (pl *PrefixLogger) Debug(arg0 string, args ...interface{}) {
	_ = root.log(levelDebug, "DEBUG", pl.pfx(arg0), args...)
}

func (pl *PrefixLogger) Info(arg0 string, args ...interface{}) {
	_ = root.log(levelInfo, "INFO", pl.pfx(arg0), args...)
}

func (pl *PrefixLogger) Warn(arg0 string, args ...interface{}) error {
	return root.log(levelWarning, "WARNING", pl.pfx(arg0), args...)
}

func (pl *PrefixLogger) Error(arg0 string, args ...interface{}) error {
	return root.log(levelError, "ERROR", pl.pfx(arg0), args...)
}

func (pl *PrefixLogger) AddLogPrefix(prefix string) {
	if len(pl.prefix) > 0 {
		pl.prefix += " "
	}

	pl.prefix += "[" + prefix + "]"
}

func (pl *PrefixLogger) ClearLogPrefixes() {
	pl.prefix = ""
}

func Debug(arg0 string, args ...interface{}) {
	_ = root.log(levelDebug, "DEBUG", arg0, args...)
}

func Info(arg0 string, args ...interface{}) {
	_ = root.log(levelInfo, "INFO", arg0, args...)
}

func Warn(arg0 string, args ...interface{}) error {
	return root.log(levelWarning, "WARNING", arg0, args...)
}

func Error(arg0 string, args ...interface{}) error {
	return root.log(levelError, "ERROR", arg0, args...)
}
