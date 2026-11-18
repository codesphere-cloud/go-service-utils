package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Numeric levels of the Codesphere JSON log format,
// the same ones the TypeScript services write.
const (
	codesphereLevelDebug   = 10
	codesphereLevelInfo    = 20
	codesphereLevelWarning = 30
	codesphereLevelError   = 40
)

// NewJSONHandler returns a handler that writes the Codesphere JSON log format to stdout:
// one JSON object per line with the fields message, date and a numeric level.
func NewJSONHandler() slog.Handler {
	return newCodesphereJSONHandler(os.Stdout)
}

func newCodesphereJSONHandler(out io.Writer) slog.Handler {
	return attrsInMessageHandler{
		inner: slog.NewJSONHandler(out, &slog.HandlerOptions{ReplaceAttr: toCodesphereField}),
	}
}

func toCodesphereField(_ []string, attr slog.Attr) slog.Attr {
	switch attr.Key {
	case slog.TimeKey:
		attr.Key = "date"
	case slog.MessageKey:
		attr.Key = "message"
	case slog.LevelKey:
		attr.Value = slog.IntValue(toCodesphereLevel(attr.Value.Any().(slog.Level)))
	}
	return attr
}

func toCodesphereLevel(level slog.Level) int {
	switch {
	case level >= slog.LevelError:
		return codesphereLevelError
	case level >= slog.LevelWarn:
		return codesphereLevelWarning
	case level >= slog.LevelInfo:
		return codesphereLevelInfo
	default:
		return codesphereLevelDebug
	}
}

// The log pipeline (Grafana Alloy) keeps only message and stack of a JSON line,
// so attributes go into the message:
//
//	logger.With("component", "api").WithGroup("db").Info("connected", "host", "pg")
//	// {"date":"…","level":20,"message":"connected component=api db.host=pg"}
type attrsInMessageHandler struct {
	inner       slog.Handler
	boundAttrs  string
	groupPrefix string
}

func (h attrsInMessageHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h attrsInMessageHandler) Handle(ctx context.Context, record slog.Record) error {
	var message strings.Builder
	message.WriteString(record.Message)
	message.WriteString(h.boundAttrs)
	record.Attrs(func(attr slog.Attr) bool {
		message.WriteString(h.formatAttr(attr))
		return true
	})
	return h.inner.Handle(ctx, slog.NewRecord(record.Time, record.Level, message.String(), record.PC))
}

func (h attrsInMessageHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	for _, attr := range attrs {
		h.boundAttrs += h.formatAttr(attr)
	}
	return h
}

func (h attrsInMessageHandler) WithGroup(name string) slog.Handler {
	h.groupPrefix += name + "."
	return h
}

func (h attrsInMessageHandler) formatAttr(attr slog.Attr) string {
	return fmt.Sprintf(" %s%s=%v", h.groupPrefix, attr.Key, attr.Value)
}
