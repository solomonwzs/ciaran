package logger

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

const (
	_CTRL_WRITE_LOG int = iota
	_CTRL_ADD_LOGGER
	_CTRL_DEL_LOGGER
)

const (
	_COLOR_RED        = "\033[31m"
	_COLOR_GREEN      = "\033[32m"
	_COLOR_YELLOW     = "\033[33m"
	_COLOR_BLUE       = "\033[34m"
	_COLOR_PURPLE     = "\033[35m"
	_COLOR_LIGHT_BLUE = "\033[36m"
	_COLOR_GRAY       = "\033[37m"
	_COLOR_BLACK      = "\033[30m"
)

const (
	_CTRL_MSG_UPPER_LIMIT = 4096
)

type LogRecord struct {
	file    string
	line    int
	message string
	created time.Time
	level   int
	format  string
}

type ctrlManager struct {
	mux         sync.Mutex
	logChannels map[string]chan *LogRecord
}

var (
	debug bool         = true
	cm    *ctrlManager = nil
)

func (l *LogRecord) setLevel(level int) {
	l.level = level
}

func newLogRecord(calldepth int, message string, level int) *LogRecord {
	l := new(LogRecord)
	l.created = time.Now()
	l.message = message
	l.level = level
	if debug {
		_, file, line, ok := runtime.Caller(calldepth)
		if !ok {
			l.file = "???"
			l.line = 0
		} else {
			l.file = file
			l.line = line
		}
	}
	return l
}

func broadcast(l *LogRecord) {
	for _, ch := range cm.logChannels {
		ch <- l
	}
}

func consoleOutput(l *LogRecord) {
	var level, color string
	switch l.Level() {
	case FINEST:
		color = _COLOR_BLACK
		level = "N"
	case FINE:
		color = _COLOR_BLUE
		level = "F"
	case DEBUG:
		color = _COLOR_GREEN
		level = "D"
	case TRACE:
		color = _COLOR_LIGHT_BLUE
		level = "T"
	case INFO:
		color = _COLOR_GRAY
		level = "I"
	case WARNING:
		color = _COLOR_YELLOW
		level = "W"
	case ERROR:
		color = _COLOR_RED
		level = "E"
	case CRITICAL:
		color = _COLOR_PURPLE
		level = "C"
	}

	file := l.File()
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}

	fmt.Printf("%s%s [%s %s:%d] \033[0m%s",
		color, l.Created().Format("2006-01-02 15:04:05"),
		level, short, l.Line(), l.Message())
}
