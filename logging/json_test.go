package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
})
