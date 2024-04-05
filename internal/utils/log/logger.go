package log

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
)

var l = setLoggerOutput(os.Stdout)

type Event struct {
	e *zerolog.Event
}

// Debug logs a message at info level.
func Debug() *Event {
	return &Event{l.Debug()}
}

// Info logs a message at info level.
func Info() *Event {
	return &Event{l.Info()}
}

// Error logs a message at error level.
func Error() *Event {
	return &Event{l.Error()}
}

// Fatal logs a message at fatal level.
func Fatal() *Event {
	return &Event{l.Fatal()}
}

// Str logs a string with the given key and value.
func (e *Event) Str(key, val string) *Event {
	return &Event{e.e.Str(key, val)}
}

// Err logs an error.
func (e *Event) Err(err error) *Event {
	return &Event{e.e.Err(err)}
}

// Msg logs a message with the given tag and message.
func (e *Event) Msg(tag, msg string) {
	if e == nil {
		return
	}

	e.e.Msg("\033[35m" + tag + "\033[0m " + msg)
}

// DNS logs a DNS message.
func (e *Event) DNS(m *dns.Msg) *Event {
	names := make([]string, 0, len(m.Question))
	ips := make([]string, 0, len(m.Answer))

	for _, que := range m.Question {
		names = append(names, que.Name)
	}

	for _, ans := range m.Answer {
		ansA, ok := ans.(*dns.A)
		if !ok {
			continue
		}

		ips = append(ips, ansA.A.String())
	}

	e = e.Str("names", strings.Join(names, ","))

	if len(ips) != 0 {
		e = e.Str("ips", strings.Join(ips, ","))
	}

	return e
}

// Msgf logs a message with the given tag and format string.
func (e *Event) Msgf(tag, format string, v ...any) {
	if e == nil {
		return
	}

	e.Msg(tag, fmt.Sprintf(format, v...))
}

// SetOutput sets the output for logging messages.
func SetOutput(out io.Writer) {
	l = setLoggerOutput(out)
}

func setLoggerOutput(out io.Writer) zerolog.Logger {
	return zlog.Output(zerolog.ConsoleWriter{
		Out: out,
		FormatFieldName: func(i any) string {
			str, ok := i.(string)
			if !ok {
				return ""
			}

			return fmt.Sprintf("\033[34m%s=\033[0m", str)
		},
		FormatFieldValue: func(i any) string {
			str, ok := i.(string)
			if !ok {
				return ""
			}

			return fmt.Sprintf("\033[34m%s\033[0m", str)
		},
		FormatErrFieldName: func(any) string {
			return ""
		},
		FormatErrFieldValue: func(i any) string {
			str, ok := i.(string)
			if !ok {
				return ""
			}

			return fmt.Sprintf("\033[31m(%s)\033[0m", str)
		},
		FormatTimestamp: func(i any) string {
			str, ok := i.(string)
			if !ok {
				return ""
			}

			parse, err := time.Parse(time.RFC3339, str)
			if err != nil {
				return ""
			}

			return fmt.Sprintf("\033[36m%s\033[0m", parse.Format("15:04:05"))
		},
	}).Level(zerolog.InfoLevel)
}
