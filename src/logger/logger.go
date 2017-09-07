package logger

import (
	"fmt"
	"time"
)

const (
	FINEST int = iota
	FINE
	DEBUG
	TRACE
	INFO
	WARNING
	ERROR
	CRITICAL
)

var (
	_SHORT_LEVEL_NAME map[int]string = map[int]string{
		FINEST:   "N",
		FINE:     "F",
		DEBUG:    "D",
		TRACE:    "T",
		INFO:     "I",
		WARNING:  "W",
		ERROR:    "E",
		CRITICAL: "C",
	}

	_LEVEL_COLOR map[int]string = map[int]string{
		FINEST:   _COLOR_BLACK,
		FINE:     _COLOR_BLUE,
		DEBUG:    _COLOR_GREEN,
		TRACE:    _COLOR_LIGHT_BLUE,
		INFO:     _COLOR_GRAY,
		WARNING:  _COLOR_YELLOW,
		ERROR:    _COLOR_RED,
		CRITICAL: _COLOR_PURPLE,
	}
)

func (l *LogRecord) Color() string {
	return _LEVEL_COLOR[l.level]
}

func (l *LogRecord) File() string {
	return l.file
}

func (l *LogRecord) SFile() string {
	short := l.file
	for i := len(l.file) - 1; i > 0; i-- {
		if l.file[i] == '/' {
			short = l.file[i+1:]
			break
		}
	}
	return short
}

func (l *LogRecord) Line() int {
	return l.line
}

func (l *LogRecord) Created() time.Time {
	return l.created
}

func (l *LogRecord) Message() string {
	if l.mFormat == "" {
		return fmt.Sprintln(l.mArgv...)
	} else {
		return fmt.Sprintf(l.mFormat, l.mArgv...)
	}
}

func (l *LogRecord) Level() int {
	return l.level
}

func (l *LogRecord) LevelSName() string {
	return _SHORT_LEVEL_NAME[l.level]
}

func Init() {
	if cm == nil {
		cm = &ctrlManager{
			logChannels: make(map[string]chan *LogRecord),
		}
	}
}

func Log(level, depth int, argv ...interface{}) {
	if cm != nil && len(cm.logChannels) != 0 {
		lr := newLogRecord(depth, level, "", argv...)
		broadcast(lr)
	}
}

func Logf(level, depth int, format string, argv ...interface{}) {
	if cm != nil && len(cm.logChannels) != 0 {
		lr := newLogRecord(depth, level, format, argv...)
		broadcast(lr)
	}
}

func AddLogger(id string, f func(*LogRecord)) {
	if cm != nil {
		_, exist := cm.logChannels[id]
		if exist {
			return
		}

		cm.mux.Lock()
		defer cm.mux.Unlock()

		ch := make(chan *LogRecord, 16)
		cm.logChannels[id] = ch
		if f == nil {
			f = ConsoleOutput
		}
		go func() {
			for l := range ch {
				f(l)
			}
		}()
	}
}

func DelLogger(id string) {
	if cm != nil {
		ch, exist := cm.logChannels[id]
		if !exist {
			return
		}

		cm.mux.Lock()
		defer cm.mux.Unlock()

		delete(cm.logChannels, id)
		close(ch)
	}
}

func Info(message ...interface{}) {
	Log(INFO, 3, message...)
}

func Infof(format string, message ...interface{}) {
	Logf(INFO, 3, format, message...)
}

func Error(message ...interface{}) {
	Log(ERROR, 3, message...)
}

func Errorf(format string, message ...interface{}) {
	Logf(ERROR, 3, format, message...)
}

func Debug(message ...interface{}) {
	Log(DEBUG, 3, message...)
}

func Debugf(format string, message ...interface{}) {
	Logf(DEBUG, 3, format, message...)
}
