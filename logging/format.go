// Package logging provides log/slog handlers for Codesphere services:
// JSON lines in the Codesphere log format for production,
// and readable text for local development.
package logging

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

// Format is the output format of the handler returned by NewHandler.
type Format string

const (
	FormatJSON Format = "json"
	FormatText Format = "text"
)

// ReadFormat reads the format from the LOG_FORMAT environment variable.
// Unset means FormatJSON.
func ReadFormat() (Format, error) {
	switch raw := Format(os.Getenv("LOG_FORMAT")); raw {
	case "", FormatJSON:
		return FormatJSON, nil
	case FormatText:
		return FormatText, nil
	default:
		return "", fmt.Errorf("invalid LOG_FORMAT %q, want json or text", raw)
	}
}

// NewHandler returns a handler that writes in the given format to stdout:
// FormatText is colored only when stdout is a terminal,
// and any other format gives the handler of NewJSONHandler.
func NewHandler(format Format) slog.Handler {
	if format == FormatText {
		return tint.NewTextHandler(os.Stdout, &tint.Options{
			TimeFormat: time.TimeOnly,
			NoColor:    !isTerminal(os.Stdout),
		})
	}
	return NewJSONHandler()
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
