package logger

import (
	"fmt"
	"os"
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

	mFormat string
	mArgv   []interface{}
}

type ctrlManager struct {
	mux         sync.Mutex
	logChannels map[string]chan *LogRecord
}

var (
	debug bool         = true
	cm    *ctrlManager = nil
	pid   int
)

func init() {
	pid = os.Getpid()
}

func (l *LogRecord) setLevel(level int) {
	l.level = level
}

func newLogRecord(calldepth int, level int, format string,
	argv ...interface{}) *LogRecord {

	l := new(LogRecord)
	l.created = time.Now()
	l.level = level
	l.mFormat = format
	l.mArgv = argv
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

func ConsoleOutput(l *LogRecord) {
	fmt.Printf("%s%d %s [%s %s:%d] \033[0m%s",
		l.Color(), pid, l.Created().Format("2006-01-02 15:04:05"),
		l.LevelSName(), l.SFile(), l.Line(), l.Message())
}
