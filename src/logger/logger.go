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

func (l *LogRecord) File() string {
	return l.file
}

func (l *LogRecord) Line() int {
	return l.line
}

func (l *LogRecord) Created() time.Time {
	return l.created
}

func (l *LogRecord) Message() string {
	return l.message
}

func (l *LogRecord) Level() int {
	return l.level
}

func Init() {
	if cm == nil {
		cm = &ctrlManager{
			logChannels: make(map[string]chan *LogRecord),
		}
	}
}

func Log(level int, message ...interface{}) {
	if cm != nil && len(cm.logChannels) != 0 {
		lr := newLogRecord(2, fmt.Sprintln(message...), level)
		broadcast(lr)
	}
}

func Logf(level int, format string, message ...interface{}) {
	if cm != nil && len(cm.logChannels) != 0 {
		lr := newLogRecord(2, fmt.Sprintf(format, message...), level)
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
			f = consoleOutput
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
