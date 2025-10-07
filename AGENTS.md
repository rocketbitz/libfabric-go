# Automation Agent Guide

This project is frequently maintained with the help of automation agents. The
notes below capture the context those agents need in order to build, test, and
ship changes safely.

## Environment Requirements
- Go 1.22 or newer with CGO enabled.
- Libfabric development headers and shared libraries installed (`pkg-config`
  should discover them). CI installs `libfabric-dev` on Debian-based images.
- Default provider is `sockets`; use `FI_PROVIDER=sockets` and set the loopback
  interface (`FI_SOCKETS_IFACE=lo` on Linux, `lo0` on macOS).
- Prefer repository-local caches when running tools: set `GOCACHE` to
  `${repo}/.cache/go-build` and `GOLANGCI_LINT_CACHE` to
  `${repo}/.cache/golangci-lint`.

## Common Commands
- `just check` → gofmt, golangci-lint, unit tests (race + coverage).
- `just integration` → runs example binaries and the MSG client E2E test under
  the sockets provider, skipping unsupported providers.
- `golangci-lint run ./...` must pass before submitting changes.
- Integration jobs in CI run `go test -tags=integration ./integration/...` with
  `LIBFABRIC_TEST_EXAMPLES=1`, `FI_PROVIDER`, and the loopback interface set.

## Code Layout
- `internal/capi/` – CGO bridge to libfabric; treat as unsafe internals.
- `fi/` – low-level Go wrappers around fabrics, domains, endpoints, AVs, queues,
  memory registration, RMA, and wait sets.
- `client/` – high-level MSG/RDM client with async handlers, tracing, and
  metrics hooks.
- `examples/` and `integration/` – runnable examples and integration tests. Keep
  them in sync when adding APIs.
- `docs/` – usage (`USAGE.md`), development workflow (`DEVELOPMENT.md`), and
  design references (`MESSAGING_DESIGN.md`).

## Testing & Providers
- Unit tests assume the sockets provider; ensure `FI_SOCKETS_IFACE` is set.
- Provider-specific tests are gated by `LIBFABRIC_TEST_*` environment variables
  and must skip (not fail) when prerequisites are unavailable.
- Avoid flaky tests—gate hardware-specific flows behind env vars and cleanly skip
  when capabilities are missing.

## Documentation Expectations
- Every exported symbol requires a GoDoc comment describing behavior and
  caveats.
- Package-level `doc.go` files explain safety constraints and provider
  considerations.
- Update `README.md`, `docs/USAGE.md`, and `docs/DEVELOPMENT.md` for notable
  features or workflow changes.

## Release Checklist
1. Run `just check` and `just integration` (plus any extra provider jobs).
2. Update `CHANGELOG.md` with the release notes.
3. Tag the release (`git tag vX.Y.Z && git push origin vX.Y.Z`).
4. Publish release notes and verify module proxy availability.

Keep this guide handy whenever automation is interacting with the repository.
