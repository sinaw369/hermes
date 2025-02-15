// File: logWriter/logWriter.go
package logWriter

import (
	"fmt"
	"io"
	"log"

	"github.com/fatih/color"
)

// Logger struct wraps a standard Go logger with color capabilities
type Logger struct {
	writer   *log.Logger
	disabled bool
}

// NewLogger creates a new Logger instance
// It accepts any io.Writer (e.g., os.Stdout, files, buffers)
// If LstdFlags is true, standard flags like timestamps will be included
func NewLogger(writer io.Writer, LstdFlags, disabled bool) *Logger {
	var flags int
	if LstdFlags {
		flags = log.LstdFlags
	} else {
		flags = 0
	}
	return &Logger{
		writer:   log.New(writer, "", flags),
		disabled: disabled,
	}
}

// Error method makes Logger implement the error interface
func (l *Logger) Error() string {
	return "Logger: error occurred"
}

// InfoString logs a formatted message in white color
func (l *Logger) InfoString(format string, a ...interface{}) {
	if !l.disabled {
		l.writer.Println(color.HiWhiteString(format, a...))
	}
}

// ErrorString logs an error message.
func (l *Logger) ErrorString(format string, a ...interface{}) {
	if !l.disabled {
		l.RedOnWhiteString("⇓ ⇓ ⇓ ⇓ ⇓ ⇓ ⇓ ⇓ ⇓ ⇓ ERROR ⇓ ⇓ ⇓ ⇓ ⇓ ⇓ ⇓ ⇓ ⇓ ⇓")
		l.writer.Println(color.HiRedString(format, a...))
		l.RedOnWhiteString("⇑ ⇑ ⇑ ⇑ ⇑ ⇑ ⇑ ⇑ ⇑ ⇑ ERROR ⇑ ⇑ ⇑ ⇑ ⇑ ⇑ ⇑ ⇑ ⇑ ⇑")
	}
}

// GreenString logs a formatted message in green color
func (l *Logger) GreenString(format string, a ...interface{}) {
	if !l.disabled {
		l.writer.Println(color.HiGreenString(format, a...))
	}
}

// BlackString logs a formatted message in black color
func (l *Logger) BlackString(format string, a ...interface{}) {
	if !l.disabled {
		l.writer.Println(color.HiBlackString(format, a...))
	}
}

// BlueString logs a formatted message in blue color
func (l *Logger) BlueString(format string, a ...interface{}) {
	if !l.disabled {
		l.writer.Println(color.HiBlueString(format, a...))
	}
}

// RedString logs a formatted message in red color
func (l *Logger) RedString(format string, a ...interface{}) {
	if !l.disabled {
		l.writer.Println(color.HiRedString(format, a...))
	}
}

// MagentaString logs a formatted message in magenta color
func (l *Logger) MagentaString(format string, a ...interface{}) {
	if !l.disabled {
		l.writer.Println(color.HiMagentaString(format, a...))
	}
}

// YellowString logs a formatted message in yellow color
func (l *Logger) YellowString(format string, a ...interface{}) {
	if !l.disabled {
		l.writer.Println(color.HiYellowString(format, a...))
	}
}

// Predefined color functions for black text on white background and red on white
var (
	blackOnWhite = color.New(color.FgBlack, color.BgHiWhite).SprintFunc()
	redOnWhite   = color.New(color.FgRed, color.BgWhite).SprintFunc()
)

// BlackOnWhiteString logs a formatted message in black text on a white background
func (l *Logger) BlackOnWhiteString(format string, a ...interface{}) {
	if !l.disabled {
		l.writer.Println(blackOnWhite(fmt.Sprintf(format, a...)))
	}
}

// RedOnWhiteString logs a formatted message in red text on a white background
func (l *Logger) RedOnWhiteString(format string, a ...interface{}) {
	if !l.disabled {
		l.writer.Println(redOnWhite(fmt.Sprintf(format, a...)))
	}
}
