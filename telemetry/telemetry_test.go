package telemetry_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/codesphere-cloud/go-service-utils/telemetry"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

type request struct {
	path string
	body []byte
}

type collector struct {
	mu       sync.Mutex
	requests []request
}

func (c *collector) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	c.mu.Lock()
	c.requests = append(c.requests, request{path: req.URL.Path, body: body})
	c.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (c *collector) Paths() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	paths := make([]string, 0, len(c.requests))
	for _, r := range c.requests {
		paths = append(paths, r.path)
	}
	return paths
}

// ResourceAttributesOfSpans returns the resource attributes of the first span export.
func (c *collector) ResourceAttributesOfSpans() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range c.requests {
		if r.path != "/v1/traces" {
			continue
		}
		var export collectortrace.ExportTraceServiceRequest
		Expect(proto.Unmarshal(r.body, &export)).To(Succeed())
		attrs := map[string]string{}
		for _, attr := range export.GetResourceSpans()[0].GetResource().GetAttributes() {
			attrs[attr.GetKey()] = attr.GetValue().GetStringValue()
		}
		return attrs
	}
	return nil
}

func initWithCollector(ctx context.Context) (*collector, func(context.Context) error) {
	recorder := &collector{}
	server := httptest.NewServer(recorder)
	DeferCleanup(server.Close)
	GinkgoT().Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", server.URL)

	shutdown, err := telemetry.Init(ctx, telemetry.Options{ServiceName: "test-service", InstanceID: "test-instance"})
	Expect(err).NotTo(HaveOccurred())
	return recorder, shutdown
}

var _ = Describe("Init", func() {
	It("endpoint without path, metrics flushed -> posts to /v1/metrics", func(ctx SpecContext) {
		recorder, shutdown := initWithCollector(ctx)
		counter, err := otel.Meter("test").Int64Counter("test_counter")
		Expect(err).NotTo(HaveOccurred())
		counter.Add(ctx, 1)

		Expect(shutdown(ctx)).To(Succeed())

		Expect(recorder.Paths()).To(ContainElement("/v1/metrics"))
	})

	It("endpoint without path, spans flushed -> posts to /v1/traces", func(ctx SpecContext) {
		recorder, shutdown := initWithCollector(ctx)
		_, span := otel.Tracer("test").Start(ctx, "test-span")
		span.End()

		Expect(shutdown(ctx)).To(Succeed())

		Expect(recorder.Paths()).To(ContainElement("/v1/traces"))
	})

	It("service name and instance id -> service.name and service.instance.id resource attributes", func(ctx SpecContext) {
		recorder, shutdown := initWithCollector(ctx)
		_, span := otel.Tracer("test").Start(ctx, "test-span")
		span.End()

		Expect(shutdown(ctx)).To(Succeed())

		attrs := recorder.ResourceAttributesOfSpans()
		Expect(attrs).To(HaveKeyWithValue("service.name", "test-service"))
		Expect(attrs).To(HaveKeyWithValue("service.instance.id", "test-instance"))
	})
})
