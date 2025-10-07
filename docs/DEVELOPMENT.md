# Development Guide

This guide documents repository structure, testing strategy, and the workflow we
follow when preparing releases of `libfabric-go`.

## Repository Layout

- `client/` – high-level client abstraction layered on top of `fi`.
- `fi/` – low-level wrappers for libfabric resources, matching the C API while
  exposing idiomatic Go helpers.
- `internal/capi/` – CGO bridge and enum/constant conversions. Only packages in
  this module should import it directly.
- `examples/` – runnable examples exercising discovery, messaging, RMA, and the
  high-level client.
- `integration/` – integration tests that run the examples and end-to-end client
  flows.
- `docs/` – design notes and usage/development documentation.
- `ROADMAP.md` – phased plan capturing upcoming work.

## Tooling

- [`just`](https://github.com/casey/just) provides a thin command runner. Common
  recipes:
  - `just build` – compile the entire module.
  - `just check` – gofmt + golangci-lint + unit tests (race + coverage).
  - `just integration` – run the integration test suite with sockets defaults.
- `golangci-lint` enforces gofmt, revive, errcheck, goimports, and other
  analyzers. Configuration lives in `.golangci.yml`.

## Testing Strategy

| Layer | Command | Notes |
| --- | --- | --- |
| Unit | `just check` | Includes the race detector and code coverage. |
| Integration | `just integration` | Executes example binaries and the MSG client e2e test. Skips when capabilities are missing. |
| Benchmarks | `go test ./... -run=^$ -bench=. -benchmem` | Captures microbenchmarks for critical paths (CircleCI runs this in the `go-bench` job). |

Providers beyond sockets are opted in via the environment variables listed in
[`docs/USAGE.md`](USAGE.md). Tests intentionally skip when a provider advertises
insufficient capabilities to avoid noisy failures on shared CI infrastructure.

## Releasing

1. Ensure the roadmap phase targeted for the release is marked complete and the
   outstanding questions capture deferred work.
2. Run `just check`, `just integration`, and any provider-specific jobs relevant
   to the release.
3. Update `CHANGELOG.md` (or create it, as in `v0.0.1`) with a summary of fixes
   and features.
4. Tag the release: `git tag v0.0.1 && git push origin v0.0.1`.
5. Publish the release notes on GitHub and, if necessary, cross-post to internal
   channels.

## Style & Documentation

- All exported identifiers must have GoDoc comments.
- Comments should focus on behaviors, contracts, and provider caveats – avoid
  describing the obvious.
- Keep examples runnable and opt into provider capabilities deliberately to
  avoid flaky CI runs.

## Filing Issues & PRs

- Include provider environment details (`FI_PROVIDER`, version numbers) when
  reporting bugs.
- Reference the roadmap phase or docs page for larger changes so context is
  preserved.
- Prefer smaller, self-contained PRs with clear testing notes.

Thank you for contributing!
