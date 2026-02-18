package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/fatih/color"
)

var (
	// Regex to match text inside brackets
	bracketRegex = regexp.MustCompile(`\[([^\]]+)\]`)

	// Color schemes for different bracket contents
	colors = map[string]*color.Color{
		// Infrastructure
		"RabbitMQ":   color.New(color.FgCyan, color.Bold),
		"Redis":      color.New(color.FgMagenta, color.Bold),
		"PostgreSQL": color.New(color.FgBlue, color.Bold),
		"Database":   color.New(color.FgBlue, color.Bold),
		"Migration":  color.New(color.FgBlue),

		// Workers
		"ScheduledMessageWorker": color.New(color.FgYellow, color.Bold),
		"MessageSender":          color.New(color.FgYellow, color.Bold),
		"RateLimiter":            color.New(color.FgYellow),

		// Business Logic
		"Schedule":    color.New(color.FgGreen, color.Bold),
		"Reschedule":  color.New(color.FgGreen, color.Bold),
		"Appointment": color.New(color.FgGreen),
		"Booking":     color.New(color.FgGreen),
		"Patient":     color.New(color.FgGreen),

		// Services
		"EMAIL":     color.New(color.FgCyan),
		"BREVO":     color.New(color.FgCyan),
		"WhatsApp":  color.New(color.FgGreen),
		"AUTH":      color.New(color.FgMagenta),
		"Settings":  color.New(color.FgBlue),
		"Calendar":  color.New(color.FgBlue),
		"Payment":   color.New(color.FgYellow),
		"Analytics": color.New(color.FgCyan),
		"Waitlist":  color.New(color.FgYellow),

		// Error/Warning/Success indicators
		"ERROR":   color.New(color.FgRed, color.Bold),
		"FATAL":   color.New(color.FgRed, color.Bold, color.BgWhite),
		"WARN":    color.New(color.FgYellow, color.Bold),
		"WARNING": color.New(color.FgYellow, color.Bold),
		"INFO":    color.New(color.FgCyan),
		"SUCCESS": color.New(color.FgGreen, color.Bold),
		"DEBUG":   color.New(color.FgWhite),
	}

	// Default color for unknown brackets
	defaultColor = color.New(color.FgCyan, color.Bold)

	// Disable colors (for testing or non-TTY environments)
	noColor = false
)

// Init configures the logger
func Init() {
	// Set default log format
	log.SetFlags(log.Ldate | log.Ltime)
	log.SetOutput(os.Stdout)

	// Force enable colors by default (can be disabled with NO_COLOR env var)
	// This ensures colors work in most terminals, even through pipes
	if os.Getenv("NO_COLOR") != "" {
		noColor = true
		color.NoColor = true
	} else {
		// Force colors enabled for better visibility
		noColor = false
		color.NoColor = false
	}
}

// DisableColor disables colored output
func DisableColor() {
	noColor = true
	color.NoColor = true
}

// EnableColor enables colored output
func EnableColor() {
	noColor = false
	color.NoColor = false
}

// SetOutput sets the output destination for the logger
func SetOutput(w io.Writer) {
	log.SetOutput(w)
}

// colorize adds colors to text inside brackets
func colorize(format string) string {
	if noColor {
		return format
	}

	return bracketRegex.ReplaceAllStringFunc(format, func(match string) string {
		// Extract content inside brackets
		content := strings.Trim(match, "[]")

		// Find color for this content
		c, exists := colors[content]
		if !exists {
			c = defaultColor
		}

		// Return colored bracket with content
		return c.Sprintf("[%s]", content)
	})
}

// Printf prints a formatted log message with colored brackets
func Printf(format string, v ...interface{}) {
	coloredFormat := colorize(format)
	log.Printf(coloredFormat, v...)
}

// Println prints a log message with colored brackets
func Println(v ...interface{}) {
	// Convert to string and colorize
	message := fmt.Sprint(v...)
	coloredMessage := colorize(message)
	log.Println(coloredMessage)
}

// Print prints a log message with colored brackets
func Print(v ...interface{}) {
	message := fmt.Sprint(v...)
	coloredMessage := colorize(message)
	log.Print(coloredMessage)
}

// Fatalf prints a fatal error message with colored brackets and exits
func Fatalf(format string, v ...interface{}) {
	coloredFormat := colorize(format)
	log.Fatalf(coloredFormat, v...)
}

// Panicf prints a panic message with colored brackets and panics
func Panicf(format string, v ...interface{}) {
	coloredFormat := colorize(format)
	log.Panicf(coloredFormat, v...)
}

// Error prints an error message with colored brackets
func Error(v ...interface{}) {
	message := fmt.Sprint(v...)
	coloredMessage := colorize("[ERROR] " + message)
	log.Print(coloredMessage)
}

// Errorf prints a formatted error message with colored brackets
func Errorf(format string, v ...interface{}) {
	coloredFormat := colorize("[ERROR] " + format)
	log.Printf(coloredFormat, v...)
}

// Warn prints a warning message with colored brackets
func Warn(v ...interface{}) {
	message := fmt.Sprint(v...)
	coloredMessage := colorize("[WARN] " + message)
	log.Print(coloredMessage)
}

// Warnf prints a formatted warning message with colored brackets
func Warnf(format string, v ...interface{}) {
	coloredFormat := colorize("[WARN] " + format)
	log.Printf(coloredFormat, v...)
}

// Info prints an info message with colored brackets
func Info(v ...interface{}) {
	message := fmt.Sprint(v...)
	coloredMessage := colorize("[INFO] " + message)
	log.Print(coloredMessage)
}

// Infof prints a formatted info message with colored brackets
func Infof(format string, v ...interface{}) {
	coloredFormat := colorize("[INFO] " + format)
	log.Printf(coloredFormat, v...)
}

// Success prints a success message with colored brackets
func Success(v ...interface{}) {
	message := fmt.Sprint(v...)
	coloredMessage := colorize("[SUCCESS] " + message)
	log.Print(coloredMessage)
}

// Successf prints a formatted success message with colored brackets
func Successf(format string, v ...interface{}) {
	coloredFormat := colorize("[SUCCESS] " + format)
	log.Printf(coloredFormat, v...)
}

// Debug prints a debug message with colored brackets
func Debug(v ...interface{}) {
	message := fmt.Sprint(v...)
	coloredMessage := colorize("[DEBUG] " + message)
	log.Print(coloredMessage)
}

// Debugf prints a formatted debug message with colored brackets
func Debugf(format string, v ...interface{}) {
	coloredFormat := colorize("[DEBUG] " + format)
	log.Printf(coloredFormat, v...)
}

// RegisterColor registers a custom color for a specific bracket content
func RegisterColor(content string, c *color.Color) {
	colors[content] = c
}

// isTerminal checks if the writer is a terminal
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return isatty(f)
	}
	return false
}

// isatty checks if a file is a terminal
func isatty(f *os.File) bool {
	var stat os.FileInfo
	var err error
	if stat, err = f.Stat(); err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}
