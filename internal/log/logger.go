package log

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
)

var l = setOutput(os.Stdout)

type Logger struct {
	zerolog.Logger
}

type Event struct {
	e *zerolog.Event
}

func Info() *Event {
	return &Event{l.Logger.Info()}
}

func Error() *Event {
	return &Event{l.Logger.Error()}
}

func Fatal() *Event {
	return &Event{l.Logger.Fatal()}
}

func (e *Event) Str(key, val string) *Event {
	return &Event{e.e.Str(key, val)}
}

func (e *Event) Err(err error) *Event {
	return &Event{e.e.Err(err)}
}

func (e *Event) Msg(tag, msg string) {
	if e == nil {
		return
	}
	e.e.Msg("\033[35m" + tag + "\033[0m " + msg)
}

func (e *Event) Msgf(tag, format string, v ...interface{}) {
	if e == nil {
		return
	}
	e.Msg(tag, fmt.Sprintf(format, v...))
}

func SetOutput(out io.Writer) {
	l = setOutput(out)
}

func setOutput(out io.Writer) Logger {
	return Logger{zlog.Output(zerolog.ConsoleWriter{
		Out: out,
		FormatFieldName: func(i interface{}) string {
			return "\033[34m" + i.(string) + "=\033[0m"
		},
		FormatFieldValue: func(i interface{}) string {
			return "\033[34m" + i.(string) + "\033[0m"
		},
		FormatErrFieldName: func(i interface{}) string {
			return ""
		},
		FormatErrFieldValue: func(i interface{}) string {
			return "\033[31m(" + i.(string) + ")\033[0m"
		},
		FormatTimestamp: func(i interface{}) string {
			parse, _ := time.Parse(time.RFC3339, i.(string))
			return "\033[36m" + parse.Format("15:04:05") + "\033[0m"
		},
	})}
}
