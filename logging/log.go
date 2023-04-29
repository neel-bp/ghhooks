package logging

import "log"

const (
	DEBUG = iota
	INFO
	WARN
	ERROR
	CRITICAL
)

type loggerT struct {
	*log.Logger
	level uint
}

var logger *loggerT
var glevel uint
var stdlog *log.Logger

func Init(l *log.Logger, level uint) {
	glevel = level
	stdlog = l
	if logger == nil {
		logger = New(l, level)
	}
}

func GetLogger() *loggerT {
	if logger == nil {
		Init(stdlog, glevel)
		return logger
	}
	return logger
}

func New(l *log.Logger, level uint) *loggerT {
	return &loggerT{
		Logger: l,
		level:  level,
	}
}

func (l loggerT) DEBUG(v ...any) {
	if l.level <= DEBUG {
		l.Print(v...)
	}
}

func (l loggerT) DEBUGf(format string, v ...any) {
	if l.level <= DEBUG {
		l.Printf(format, v...)
	}
}

func (l loggerT) DEBUGln(v ...any) {
	if l.level <= DEBUG {
		l.Println(v...)
	}
}

func (l loggerT) INFO(v ...any) {
	if l.level <= INFO {
		l.Print(v...)
	}
}

func (l loggerT) INFOf(format string, v ...any) {
	if l.level <= INFO {
		l.Printf(format, v...)
	}
}

func (l loggerT) INFOln(v ...any) {
	if l.level <= INFO {
		l.Println(v...)
	}
}

func (l loggerT) WARN(v ...any) {
	if l.level <= WARN {
		l.Print(v...)
	}
}

func (l loggerT) WARNf(format string, v ...any) {
	if l.level <= WARN {
		l.Printf(format, v...)
	}
}

func (l loggerT) WARNln(v ...any) {
	if l.level <= WARN {
		l.Println(v...)
	}
}

func (l loggerT) ERROR(v ...any) {
	if l.level <= ERROR {
		l.Print(v...)
	}
}

func (l loggerT) ERRORf(format string, v ...any) {
	if l.level <= ERROR {
		l.Printf(format, v...)
	}
}

func (l loggerT) ERRORln(v ...any) {
	if l.level <= ERROR {
		l.Println(v...)
	}
}

func (l loggerT) CRITICAL(v ...any) {
	if l.level <= CRITICAL {
		l.Print(v...)
	}
}

func (l loggerT) CRITICALf(format string, v ...any) {
	if l.level <= CRITICAL {
		l.Printf(format, v...)
	}
}

func (l loggerT) CRITICALln(v ...any) {
	if l.level <= CRITICAL {
		l.Println(v...)
	}
}
