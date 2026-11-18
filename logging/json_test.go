package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel/trace"
)

var _ = Describe("codesphere JSON handler", func() {
	var (
		out    *bytes.Buffer
		logger *slog.Logger
	)

	BeforeEach(func() {
		out = &bytes.Buffer{}
		logger = slog.New(newCodesphereJSONHandler(out))
	})

	decodeLine := func() map[string]any {
		var line map[string]any
		Expect(json.Unmarshal(out.Bytes(), &line)).To(Succeed())
		return line
	}

	It("info record -> message, date and level 20 fields", func() {
		logger.Info("listening")

		line := decodeLine()
		Expect(line).To(HaveKeyWithValue("message", "listening"))
		Expect(line).To(HaveKeyWithValue("level", float64(codesphereLevelInfo)))
		Expect(line).To(HaveKey("date"))
		Expect(line).NotTo(HaveKey("msg"))
		Expect(line).NotTo(HaveKey("time"))
	})

	It("warn record -> level 30", func() {
		logger.Warn("slow")

		Expect(decodeLine()).To(HaveKeyWithValue("level", float64(codesphereLevelWarning)))
	})

	It("error record -> level 40", func() {
		logger.Error("failed")

		Expect(decodeLine()).To(HaveKeyWithValue("level", float64(codesphereLevelError)))
	})

	It("level between warn and error -> warning level", func() {
		logger.Log(GinkgoT().Context(), slog.LevelWarn+1, "odd")

		Expect(decodeLine()).To(HaveKeyWithValue("level", float64(codesphereLevelWarning)))
	})

	It("debug record with debug enabled -> level 10", func() {
		handler := attrsInMessageHandler{inner: slog.NewJSONHandler(out, &slog.HandlerOptions{
			Level:       slog.LevelDebug,
			ReplaceAttr: toCodesphereField,
		})}

		slog.New(handler).Debug("trace")

		Expect(decodeLine()).To(HaveKeyWithValue("level", float64(codesphereLevelDebug)))
	})

	It("record with attributes -> attributes folded into message, not separate fields", func() {
		logger.Info("listening", "addr", ":8080")

		line := decodeLine()
		Expect(line).To(HaveKeyWithValue("message", "listening addr=:8080"))
		Expect(line).NotTo(HaveKey("addr"))
	})

	It("logger with bound attributes -> bound attributes before record attributes", func() {
		logger.With("component", "api").Info("request", "status", 200)

		Expect(decodeLine()).To(HaveKeyWithValue("message", "request component=api status=200"))
	})

	It("logger with group -> attribute keys prefixed by group", func() {
		logger.WithGroup("db").Info("connected", "host", "pg")

		line := decodeLine()
		Expect(line).To(HaveKeyWithValue("message", "connected db.host=pg"))
		Expect(line).NotTo(HaveKey("db"))
	})

	Describe("attribute values", func() {
		It("value without spaces, equals signs, quotes or control characters -> unquoted", func() {
			logger.Info("login", "user", "jane")

			Expect(decodeLine()).To(HaveKeyWithValue("message", "login user=jane"))
		})

		It("error with spaces -> quoted, so the next attribute stays separate", func() {
			logger.Error("failed", "error", errors.New("dial tcp: connection refused"), "user", "x")

			Expect(decodeLine()).To(HaveKeyWithValue("message", `failed error="dial tcp: connection refused" user=x`))
		})

		It("value forging a key=value pair -> quoted, so it stays one value", func() {
			logger.Info("x", "name", "a level=40 b=c")

			Expect(decodeLine()).To(HaveKeyWithValue("message", `x name="a level=40 b=c"`))
		})

		It("value with a newline and quotes -> escaped inside quotes", func() {
			logger.Info("x", "name", "1\n\"level\":40")

			Expect(decodeLine()).To(HaveKeyWithValue("message", `x name="1\n\"level\":40"`))
		})

		It("empty value -> empty quotes", func() {
			logger.Info("x", "name", "")

			Expect(decodeLine()).To(HaveKeyWithValue("message", `x name=""`))
		})

		It("record attribute with a redacting LogValuer -> redacted value", func() {
			logger.Info("login", "token", redactedToken("hunter2"))

			Expect(decodeLine()).To(HaveKeyWithValue("message", "login token=[REDACTED]"))
		})

		It("bound attribute with a redacting LogValuer -> redacted value", func() {
			logger.With("token", redactedToken("hunter2")).Info("login")

			Expect(decodeLine()).To(HaveKeyWithValue("message", "login token=[REDACTED]"))
		})
	})

	Describe("groups", func() {
		It("group attribute -> members flattened with the group as prefix", func() {
			logger.Info("connected", slog.Group("db", "host", "pg", "port", 5432))

			Expect(decodeLine()).To(HaveKeyWithValue("message", "connected db.host=pg db.port=5432"))
		})

		It("group attribute under a logger group -> both prefixes", func() {
			logger.WithGroup("app").Info("connected", slog.Group("db", "host", "pg"))

			Expect(decodeLine()).To(HaveKeyWithValue("message", "connected app.db.host=pg"))
		})

		It("group attribute holding a redacting LogValuer -> member redacted and prefixed", func() {
			logger.Info("login", slog.Group("client", "token", redactedToken("hunter2"), "user", "x"))

			Expect(decodeLine()).To(HaveKeyWithValue("message", "login client.token=[REDACTED] client.user=x"))
		})

		It("LogValuer resolving to a group -> members flattened with the key as prefix", func() {
			logger.Info("login", "client", client{user: "x"})

			Expect(decodeLine()).To(HaveKeyWithValue("message", "login client.user=x client.token=[REDACTED]"))
		})

		It("group attribute without a name -> members inlined", func() {
			logger.Info("x", slog.Group("", "a", 1))

			Expect(decodeLine()).To(HaveKeyWithValue("message", "x a=1"))
		})

		It("empty group attribute -> left out", func() {
			logger.Info("x", slog.Group("db"))

			Expect(decodeLine()).To(HaveKeyWithValue("message", "x"))
		})

		It("handler group with an empty name -> handler unchanged", func() {
			slog.New(newCodesphereJSONHandler(out).WithGroup("")).Info("connected", "host", "pg")

			Expect(decodeLine()).To(HaveKeyWithValue("message", "connected host=pg"))
		})
	})

	Describe("trace correlation", func() {
		spanContext := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    trace.TraceID{0x0a, 0xf7, 0x65, 0x19, 0x16, 0xcd, 0x43, 0xdd, 0x84, 0x48, 0xeb, 0x21, 0x1c, 0x80, 0x31, 0x9c},
			SpanID:     trace.SpanID{0xb7, 0xad, 0x6b, 0x71, 0x69, 0x20, 0x33, 0x31},
			TraceFlags: trace.FlagsSampled,
		})

		It("span in context -> traceId and spanId as top-level hex fields", func() {
			ctx := trace.ContextWithSpanContext(GinkgoT().Context(), spanContext)

			logger.InfoContext(ctx, "connected")

			line := decodeLine()
			Expect(line).To(HaveKeyWithValue("message", "connected"))
			Expect(line).To(HaveKeyWithValue("traceId", "0af7651916cd43dd8448eb211c80319c"))
			Expect(line).To(HaveKeyWithValue("spanId", "b7ad6b7169203331"))
		})

		It("span in context under a group -> trace ids stay top-level, outside the message", func() {
			ctx := trace.ContextWithSpanContext(GinkgoT().Context(), spanContext)

			logger.WithGroup("db").InfoContext(ctx, "connected", "host", "pg")

			line := decodeLine()
			Expect(line).To(HaveKeyWithValue("message", "connected db.host=pg"))
			Expect(line).To(HaveKeyWithValue("traceId", "0af7651916cd43dd8448eb211c80319c"))
			Expect(line).NotTo(HaveKey("db"))
		})

		It("no span in context -> no trace fields", func() {
			logger.InfoContext(GinkgoT().Context(), "connected")

			line := decodeLine()
			Expect(line).NotTo(HaveKey("traceId"))
			Expect(line).NotTo(HaveKey("spanId"))
		})

		It("invalid span context -> no trace fields", func() {
			ctx := trace.ContextWithSpanContext(GinkgoT().Context(), trace.SpanContext{})

			logger.InfoContext(ctx, "connected")

			Expect(decodeLine()).NotTo(HaveKey("traceId"))
		})
	})

	Describe("empty attributes", func() {
		It("zero attribute -> left out", func() {
			logger.Info("x", slog.Attr{})

			Expect(decodeLine()).To(HaveKeyWithValue("message", "x"))
		})

		It("attribute with an empty key -> left out", func() {
			logger.Info("x", "", "v")

			Expect(decodeLine()).To(HaveKeyWithValue("message", "x"))
		})

		It("bound zero attribute -> left out", func() {
			logger.With(slog.Attr{}).Info("x")

			Expect(decodeLine()).To(HaveKeyWithValue("message", "x"))
		})
	})
})

type redactedToken string

func (redactedToken) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}

type client struct {
	user string
}

func (c client) LogValue() slog.Value {
	return slog.GroupValue(slog.String("user", c.user), slog.Any("token", redactedToken("hunter2")))
}
