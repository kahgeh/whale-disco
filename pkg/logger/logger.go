package logger

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LogLevel uint8

const (
	NormalLogLevel LogLevel = 1
	DebugLogLevel  LogLevel = 2
)

const ExitFailureStatus = 3

type Logger struct {
	sugaredLogger *zap.SugaredLogger
	level         LogLevel
	LogDone       func()
}

type loggerState struct {
	baseLogger    *zap.Logger
	sugaredLogger *zap.SugaredLogger
	names         []string
	level         LogLevel
}

var state *loggerState

func newConsoleEncoderConfig(callerKey string) zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		// Keys can be anything except the empty string.
		TimeKey:        zapcore.OmitKey,
		LevelKey:       zapcore.OmitKey,
		NameKey:        "N",
		CallerKey:      callerKey,
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

func newConsoleConfig(level zapcore.Level, callerKey string) zap.Config {
	return zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      true,
		Encoding:         "console",
		EncoderConfig:    newConsoleEncoderConfig(callerKey),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
}

func Initialise(level LogLevel) {
	if level == DebugLogLevel {
		detailedLoggerBase, _ := newConsoleConfig(zapcore.DebugLevel, "caller").Build()
		state = &loggerState{
			baseLogger:    detailedLoggerBase,
			sugaredLogger: detailedLoggerBase.WithOptions(zap.AddCallerSkip(1)).Sugar(),
			level:         level,
		}
		return
	}

	detailedLoggerBase, _ := newConsoleConfig(zapcore.InfoLevel, zapcore.OmitKey).Build()
	state = &loggerState{
		baseLogger:    detailedLoggerBase,
		sugaredLogger: detailedLoggerBase.WithOptions(zap.AddCallerSkip(1)).Sugar(),
		level:         level,
	}
	return
}

func Sync() {
	if state.level > NormalLogLevel {
		state.baseLogger.Sync()
		state.sugaredLogger.Sync()
	}
}

func toLower(words []string) []string {
	var lowerCasedWords []string
	for _, word := range words {
		lowerCasedWords = append(lowerCasedWords, strings.ToLower(word))
	}
	return lowerCasedWords
}

func getName() string {
	fullName := strings.Join(state.names, ".")
	if len(fullName) > 20 && len(state.names) > 1 {
		lastName := state.names[len(state.names)-1]
		placeHolders := strings.Repeat(".", len(state.names)-1)
		fullName = fmt.Sprintf("%s%s", placeHolders, lastName)
	}
	padding := ""
	columnWidth := 20
	if len(fullName) < columnWidth {
		padding = strings.Repeat(" ", columnWidth-len(fullName))
	}
	return fmt.Sprintf("[%s]%s", fullName, padding)
}

func New(name string) *Logger {
	opName := toPresentParticiple(toWords(name))
	state.names = append(state.names, name)
	sugaredLogger := state.sugaredLogger.Named(getName())
	if state.level == NormalLogLevel {
		return &Logger{
			level:         state.level,
			sugaredLogger: sugaredLogger,
			LogDone: func() {
				removeLoggerName()
			},
		}
	}

	if state.level == DebugLogLevel {
		sugaredLogger.Debugf("%s", opName)
	}
	return &Logger{
		level:         state.level,
		sugaredLogger: sugaredLogger,
		LogDone: func() {
			if state.level == DebugLogLevel {
				sugaredLogger.Debugf("done %s", opName)
			}
			removeLoggerName()
		},
	}
}

func removeLoggerName() {
	if len(state.names) > 0 {
		state.names = state.names[:len(state.names)-1]
	}
}

func (logger *Logger) Infof(template string, args ...interface{}) {

	logger.sugaredLogger.Infof(template, args...)
}

func (logger *Logger) Info(args ...interface{}) {

	logger.sugaredLogger.Info(args...)
}

func (logger *Logger) Debugf(template string, args ...interface{}) {

	logger.sugaredLogger.Debugf(template, args...)
}

func (logger *Logger) Debug(args ...interface{}) {

	logger.sugaredLogger.Debug(args...)
}

func (logger *Logger) Fail(args ...interface{}) {
	logger.sugaredLogger.Error(args...)
	os.Exit(ExitFailureStatus)
}

func (logger *Logger) Failf(template string, args ...interface{}) {
	logger.sugaredLogger.Errorf(template, args)
	os.Exit(ExitFailureStatus)
}

func (logger *Logger) Warn(args ...interface{}) {
	logger.sugaredLogger.Warn(args)
}

func (logger *Logger) Warnf(template string, args ...interface{}) {
	logger.sugaredLogger.Warnf(template, args)
}

func (logger *Logger) Errorf(template string, args ...interface{}) {
	logger.sugaredLogger.Errorf(template, args)
}
