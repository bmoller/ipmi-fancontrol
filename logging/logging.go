package logging

import (
	"fmt"
	"os"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

//go:generate stringer -type=LogLevel

var (
	logLevel LogLevel
)

func SetLevel(l LogLevel) {
	logLevel = l
}

func emitf(level LogLevel, format string, a ...any) (n int, err error) {
	if level >= logLevel {
		s := fmt.Sprintf("[%s] ", level) + fmt.Sprintf(format, a...)
		if s[len(s)-1] == '\n' {
			n, err = fmt.Print(s)
		} else {
			n, err = fmt.Println(s)
		}
	}

	return
}

func emitln(level LogLevel, a ...any) (n int, err error) {
	if level >= logLevel {
		n, err = fmt.Printf("[%s] %s\n", level, fmt.Sprint(a...))
	}

	return
}

func Debugf(format string, a ...any) (n int, err error) {
	return emitf(DEBUG, format, a...)
}

func Debugln(a ...any) (n int, err error) {
	return emitln(DEBUG, a...)
}

func Infof(format string, a ...any) (n int, err error) {
	return emitf(INFO, format, a...)
}

func Infoln(a ...any) (n int, err error) {
	return emitln(INFO, a...)
}

func Warnf(format string, a ...any) (n int, err error) {
	return emitf(WARN, format, a...)
}

func Warnln(a ...any) (n int, err error) {
	return emitln(WARN, a...)
}

func Errorf(format string, a ...any) (n int, err error) {
	return emitf(ERROR, format, a...)
}

func Errorln(a ...any) (n int, err error) {
	return emitln(ERROR, a...)
}

func Fatalf(format string, a ...any) {
	emitf(FATAL, format, a...)
	os.Exit(1)
}

func Fatalln(a ...any) {
	emitln(FATAL, a...)
	os.Exit(1)
}
