# go-service-utils

Shared Go utilities for Codesphere services.

```sh
go get github.com/codesphere-cloud/go-service-utils/<package>
```

The repository is public so that services can import it without `GOPRIVATE` or git credentials.
Do not put anything internal in it: code, comments, test data and commit messages are all public.

## Packages

- [`logging`](logging): `log/slog` handlers, JSON lines in the Codesphere log format with the trace and span id of the record, or readable text for local development, selected by `LOG_FORMAT`.
- [`telemetry`](telemetry): OpenTelemetry tracer and meter providers that export via OTLP/HTTP to the endpoint in `OTEL_EXPORTER_OTLP_ENDPOINT`, and W3C Trace Context propagation.

## Development

```sh
make test-unit
make lint
```

## License

Apache License 2.0, see [LICENSE](LICENSE).
