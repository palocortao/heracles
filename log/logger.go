package log

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/logrusorgru/aurora"
)

type logger struct {
	mutex        sync.Mutex
	out          io.Writer
	err          io.Writer
	buffer       []byte
	remote       string
	consoleLevel int
	remoteLevel  int
	service      string
}

// Message represents a log line to be sent to the log service
type Message struct {
	Uuid      string `json:"uuid"`
	Service   string `json:"service"`
	Position  string `json:"position"`
	Level     int    `json:"level"`
	Timestamp string `json:"timestamp"`
	Text      string `json:"text"`
	Type      string `json:"type"` //added to map in the future with the legacy log
}

const (
	OFF    = 0
	PANIC  = 1
	ERROR  = 2
	INFO   = 3
	DEBUG  = 4
	TRACE  = 5
	ALL    = 10
	FORMAT = "2006/01/02 15:04:05"
)

var instance *logger

func init() {
	var remote string
	var consoleLevel int
	var remoteLevel int

	remote, ok := os.LookupEnv("LOG_REMOTE")

	consoleLevelParam, ok := os.LookupEnv("LOG_CONSOLE_LEVEL")
	if !ok || getLevelValue(nil, consoleLevelParam) == -1 {
		consoleLevel = INFO
	} else {
		consoleLevel = getLevelValue(nil, consoleLevelParam)
	}

	remoteLevelParam, ok := os.LookupEnv("LOG_REMOTE_LEVEL")
	if !ok || getLevelValue(nil, remoteLevelParam) == -1 {
		remoteLevel = INFO
	} else {
		remoteLevel = getLevelValue(nil, remoteLevelParam)
	}

	instance = &logger{
		mutex:        sync.Mutex{},
		out:          os.Stdout,
		err:          os.Stderr,
		remote:       remote,
		consoleLevel: consoleLevel,
		remoteLevel:  remoteLevel,
	}
}

// SetLevel changes the default logger's logging levels for console and remote output
func SetLevel(ctx context.Context, consoleLevel int, remoteLevel int) {
	instance.level(ctx, consoleLevel, remoteLevel)
}

// Panic logs a message with Panic level using the default logger before calling panic()
func Panic(ctx context.Context, v ...interface{}) {
	instance.panic(ctx, v...)
}

// Panicf logs a formatted message with Panic level using the default logger before calling panic()
func Panicf(ctx context.Context, format string, v ...interface{}) {
	instance.panicf(ctx, format, v...)
}

// Error logs a message with Error level using the default logger
func Error(ctx context.Context, v ...interface{}) error {
	return instance.error(ctx, v...)
}

// Errorf logs a formatted message with Panic level using the default logger
func Errorf(ctx context.Context, format string, v ...interface{}) error {
	return instance.errorf(ctx, format, v...)
}

// Info logs a message with Info level using the default logger
func Info(ctx context.Context, v ...interface{}) {
	instance.info(ctx, v...)
}

// Infof logs a formatted message with Panic level using the default logger
func Infof(ctx context.Context, format string, v ...interface{}) {
	instance.infof(ctx, format, v...)
}

// Debug logs a message with Debug level using the default logger
func Debug(ctx context.Context, v ...interface{}) {
	instance.debug(ctx, v...)
}

// Debugf logs a formatted message with Panic level using the default logger
func Debugf(ctx context.Context, format string, v ...interface{}) {
	instance.debugf(ctx, format, v...)
}

// Trace logs a message with Trace level using the default logger
func Trace(ctx context.Context, v ...interface{}) {
	instance.trace(ctx, v...)
}

// Tracef logs a formatted message with Panic level using the default logger
func Tracef(ctx context.Context, format string, v ...interface{}) {
	instance.tracef(ctx, format, v...)
}

// ConsoleLevel returns the default logger's console output level
func ConsoleLevel(ctx context.Context) int {
	return instance.consoleLevel
}

// RemoteLevel returns the default logger's remote output level
func RemoteLevel(ctx context.Context) int {
	return instance.remoteLevel
}

// Service is a convenience method so services can register their names to be included in log messages
func Service(ctx context.Context, service string) {
	instance.service = service
}

func (logger *logger) panic(ctx context.Context, v ...interface{}) {
	if PANIC <= logger.maxLevel(ctx) {
		logger.output(ctx, PANIC, fmt.Sprintln(v...))
	}
	panic(fmt.Sprint(v...))
}

func (logger *logger) panicf(ctx context.Context, format string, v ...interface{}) {
	if PANIC <= logger.maxLevel(ctx) {
		logger.output(ctx, PANIC, fmt.Sprintf(format, v...))
	}
	panic(fmt.Sprint(v...))
}

func (logger *logger) error(ctx context.Context, v ...interface{}) error {
	err := errors.New(fmt.Sprintln(v...))
	if ERROR <= logger.maxLevel(ctx) {
		logger.output(ctx, ERROR, err.Error())
	}
	return err
}

func (logger *logger) errorf(ctx context.Context, format string, v ...interface{}) error {
	err := errors.New(fmt.Sprintf(format, v...))
	if ERROR <= logger.maxLevel(ctx) {
		logger.output(ctx, ERROR, err.Error())
	}
	return err
}

func (logger *logger) info(ctx context.Context, v ...interface{}) {
	if INFO <= logger.maxLevel(ctx) {
		logger.outputWithContext(ctx, INFO, fmt.Sprintln(v...))
	}
}

func (logger *logger) infof(ctx context.Context, format string, v ...interface{}) {
	if INFO <= logger.maxLevel(ctx) {
		logger.output(ctx, INFO, fmt.Sprintf(format, v...))
	}
}

func (logger *logger) debug(ctx context.Context, v ...interface{}) {
	if DEBUG <= logger.maxLevel(ctx) {
		logger.output(ctx, DEBUG, fmt.Sprintln(v...))
	}
}

func (logger *logger) debugf(ctx context.Context, format string, v ...interface{}) {
	if DEBUG <= logger.maxLevel(ctx) {
		logger.output(ctx, DEBUG, fmt.Sprintf(format, v...))
	}
}

func (logger *logger) trace(ctx context.Context, v ...interface{}) {
	if TRACE <= logger.maxLevel(ctx) {
		logger.output(ctx, TRACE, fmt.Sprintln(v...))
	}
}

func (logger *logger) tracef(ctx context.Context, format string, v ...interface{}) {
	if TRACE <= logger.maxLevel(ctx) {
		logger.output(ctx, TRACE, fmt.Sprintf(format, v...))
	}
}

func (logger *logger) level(ctx context.Context, consoleLevel int, remoteLevel int) {
	instance.consoleLevel = consoleLevel
	instance.remoteLevel = remoteLevel
}

func (logger *logger) output(ctx context.Context, level int, text string) {
	message := Message{
		Service:   logger.service,
		Position:  getPosition(ctx),
		Level:     level,
		Timestamp: time.Now().Format(FORMAT),
		Text:      text,
	}

	if level <= logger.consoleLevel {
		logger.consoleOutput(ctx, message)
	}

}

func (logger *logger) outputWithContext(ctx context.Context, level int, text string) {
	requUid := "" //TODO tracing.GetTracingUuid(ctx)
	message := Message{
		Uuid:      requUid,
		Service:   logger.service,
		Position:  getPosition(ctx),
		Level:     level,
		Timestamp: time.Now().Format(FORMAT),
		Text:      text,
	}

	if level <= logger.consoleLevel {
		logger.consoleOutput(ctx, message)
	}

}

func (logger *logger) consoleOutput(ctx context.Context, message Message) {
	//text := fmt.Sprintf("%s - %s - %s || %s", aurora.Green(message.Timestamp), aurora.Blue(message.Uuid), GetColorLevelText(ctx, message.Level), message.Text)
	text := fmt.Sprintf("%s - %s - %s - %s || %s", aurora.Green(message.Timestamp), message.Uuid, aurora.Blue(message.Position), GetColorLevelText(ctx, message.Level), message.Text)

	logger.mutex.Lock()
	defer logger.mutex.Unlock()
	logger.buffer = logger.buffer[:0]
	logger.buffer = append(logger.buffer, text...)
	if message.Level > ERROR {
		_, _ = logger.out.Write(logger.buffer)
	} else {
		_, _ = logger.err.Write(logger.buffer)
	}
}

func (logger *logger) maxLevel(ctx context.Context) (maxLevel int) {
	if logger.remoteLevel >= logger.consoleLevel {
		return logger.remoteLevel
	}

	return logger.consoleLevel
}

func getPosition(ctx context.Context) (position string) {
	pc, file, line, ok := runtime.Caller(4)
	if !ok {
		return "Unknown caller -"
	}

	funcName := ""

	function := runtime.FuncForPC(pc)
	if function != nil {
		funcName = function.Name() + "()"
		funcName = strings.Split(funcName, ".")[len(strings.Split(funcName, "."))-1]
	}

	position = fmt.Sprintf("%s:%d %s", file, line, funcName)

	return position
}

// GetLevelText returns the string corresponding to the input log level
func GetColorLevelText(ctx context.Context, level int) (result aurora.Value) {
	switch level {
	case PANIC:
		result = aurora.BgRed(aurora.Black("PANIC"))
	case ERROR:
		result = aurora.BgRed(aurora.Black("ERROR"))
	case INFO:
		result = aurora.White("INFO")
	case DEBUG:
		result = aurora.Cyan("DEBUG")
	case TRACE:
		result = aurora.Cyan("TRACE")
	}

	return result
}

// GetLevelText returns the string corresponding to the input log level
func GetLevelText(ctx context.Context, level int) (result string) {
	switch level {
	case PANIC:
		result = "PANIC"
	case ERROR:
		result = "ERROR"
	case INFO:
		result = "INFO"
	case DEBUG:
		result = "DEBUG"
	case TRACE:
		result = "TRACE"
	}

	return result
}

func getLevelValue(ctx context.Context, level string) (result int) {
	switch level {
	case "OFF":
		return OFF
	case "PANIC":
		return PANIC
	case "ERROR":
		return ERROR
	case "INFO":
		return INFO
	case "DEBUG":
		return DEBUG
	case "TRACE":
		return TRACE
	case "ALL":
		return ALL
	default:
		return -1
	}
}
