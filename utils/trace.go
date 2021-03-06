package utils

import (
	"io"
	"log"
	"os"
)

const CBBROKER_TRACE = "CBBROKER_TRACE"

type Printer interface {
	Print(v ...interface{})
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

type nullLogger struct{}

func (*nullLogger) Print(v ...interface{})                 {}
func (*nullLogger) Printf(format string, v ...interface{}) {}
func (*nullLogger) Println(v ...interface{})               {}

var stdOut io.Writer = os.Stdout
var Logger Printer

func init() {
	Logger = NewLogger()
}

func SetStdout(s io.Writer) {
	stdOut = s
}

func NewLogger() Printer {
	cf_trace := os.Getenv(CBBROKER_TRACE)
	switch cf_trace {
	case "", "false":
		return new(nullLogger)
	case "true":
		return newStdoutLogger()
	default:
		return newFileLogger(cf_trace)
	}
}

func newStdoutLogger() Printer {
	return log.New(stdOut, "", 0)
}

func newFileLogger(path string) Printer {
	file, err := OpenFile(path)
	if err != nil {
		logger := newStdoutLogger()
		logger.Printf("CF_TRACE ERROR CREATING LOG FILE %s:\n%s", path, err)
		return logger
	}

	return log.New(file, "", 0)
}
