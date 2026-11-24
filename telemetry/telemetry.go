// Package telemetry sets up OpenTelemetry tracing and metrics for Codesphere services.
package telemetry

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Options describe the service that Init sets up telemetry for.
type Options struct {
	// ServiceName is the service.name resource attribute of every span and metric,
	// for example "quota-service".
	ServiceName string
	// InstanceID is the service.instance.id resource attribute.
	// It must be unique per replica, so that metrics of two replicas stay separate series.
	// The pod name, which is the hostname in Kubernetes, fits.
	InstanceID string
	// Views change how the meter provider aggregates instruments,
	// for example an exponential histogram for a duration that spans seconds to hours.
	// Empty keeps the SDK defaults.
	Views []sdkmetric.View
}

// Init sets the global OpenTelemetry tracer and meter providers,
// which otel.Tracer and otel.Meter return from then on.
// Both export via OTLP/HTTP.
// The exporters read their configuration from the OTEL_EXPORTER_OTLP_* environment variables.
// They append the per-signal path, /v1/traces and /v1/metrics, to OTEL_EXPORTER_OTLP_ENDPOINT.
// Spans are exported in batches, metrics periodically.
//
// The returned shutdown flushes pending telemetry and stops the exporters.
// Call it before the process exits, or the last batch is lost.
func Init(ctx context.Context, opts Options) (shutdown func(context.Context) error, err error) {
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(opts.ServiceName),
		semconv.ServiceInstanceID(opts.InstanceID),
	))
	if err != nil {
		return nil, fmt.Errorf("failed to create OTel resource: %w", err)
	}

	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}
	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter),
	)
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithView(opts.Views...),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)

	return func(ctx context.Context) error {
		return errors.Join(tracerProvider.Shutdown(ctx), meterProvider.Shutdown(ctx))
	}, nil
}
