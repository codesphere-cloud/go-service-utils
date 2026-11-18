# go-service-utils

Shared Go utilities for Codesphere services.

```sh
go get github.com/codesphere-cloud/go-service-utils/<package>
```

The repository is public so that services can import it without `GOPRIVATE` or git credentials.
Do not put anything internal in it: code, comments, test data and commit messages are all public.

## Packages

- [`logging`](logging): `log/slog` handlers, JSON lines in the Codesphere log format, or readable text for local development, selected by `LOG_FORMAT`.

## Development

```sh
make test-unit
make lint
```

## License

Apache License 2.0, see [LICENSE](LICENSE).
