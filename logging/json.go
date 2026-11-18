package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.opentelemetry.io/otel/trace"
)

// Numeric levels of the Codesphere JSON log format,
// the same ones the TypeScript services write.
const (
	codesphereLevelDebug   = 10
	codesphereLevelInfo    = 20
	codesphereLevelWarning = 30
	codesphereLevelError   = 40
)

// Field names of the span a record was logged in,
// the ones the TypeScript services write.
const (
	traceIdKey = "traceId"
	spanIdKey  = "spanId"
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
// plus traceId and spanId as metadata,
// so attributes go into the message,
// and the ids of the span in the record's context get fields of their own:
//
//	logger.With("component", "api").WithGroup("db").InfoContext(ctx, "connected", "host", "pg", "user", "jane doe")
//	// {"date":"…","level":20,"message":"connected component=api db.host=pg db.user=\"jane doe\"","traceId":"…","spanId":"…"}
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
		writeAttr(&message, h.groupPrefix, attr)
		return true
	})
	folded := slog.NewRecord(record.Time, record.Level, message.String(), record.PC)
	folded.AddAttrs(traceAttrs(ctx)...)
	return h.inner.Handle(ctx, folded)
}

func (h attrsInMessageHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var bound strings.Builder
	bound.WriteString(h.boundAttrs)
	for _, attr := range attrs {
		writeAttr(&bound, h.groupPrefix, attr)
	}
	h.boundAttrs = bound.String()
	return h
}

// An empty name returns the handler unchanged, as the slog.Handler contract requires.
func (h attrsInMessageHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h.groupPrefix += name + "."
	return h
}

// writeAttr follows the slog.Handler contract and slog's text handler:
// values are resolved first, so a type whose LogValue redacts it stays redacted,
// groups are flattened into prefixed keys,
// groups without a name are inlined,
// attributes without a key are left out,
// and values are quoted when they would otherwise be ambiguous.
func writeAttr(message *strings.Builder, prefix string, attr slog.Attr) {
	value := attr.Value.Resolve()
	if value.Kind() == slog.KindGroup {
		if attr.Key != "" {
			prefix += attr.Key + "."
		}
		for _, member := range value.Group() {
			writeAttr(message, prefix, member)
		}
		return
	}
	if attr.Key == "" {
		return
	}
	message.WriteString(" ")
	message.WriteString(prefix)
	message.WriteString(attr.Key)
	message.WriteString("=")
	message.WriteString(quoteIfNeeded(value.String()))
}

// Values are quoted by the same rule as slog's text handler,
// so a value with spaces stays one value,
// and a value that comes from a client cannot forge key=value pairs or line breaks.
func quoteIfNeeded(value string) string {
	needsQuoting := value == "" || !utf8.ValidString(value) || strings.IndexFunc(value, func(r rune) bool {
		return r == ' ' || r == '=' || r == '"' || !unicode.IsPrint(r)
	}) >= 0
	if needsQuoting {
		return strconv.Quote(value)
	}
	return value
}

func traceAttrs(ctx context.Context) []slog.Attr {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return nil
	}
	return []slog.Attr{
		slog.String(traceIdKey, spanContext.TraceID().String()),
		slog.String(spanIdKey, spanContext.SpanID().String()),
	}
}
