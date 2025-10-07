# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
and adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.0.1] - 2025-10-07
### Added
- Low-level `fi` wrappers for discovery, queueing, messaging, memory registration
  (including MR pools) and RMA helpers.
- High-level `client` package with MSG listener/connect, asynchronous handlers,
  metrics hooks (Prometheus, OpenTelemetry), and tracing integration points.
- Runnable examples for messaging, RMA, and high-level client flows.
- Integration suite that executes the examples and a MSG client e2e test with
  sockets defaults.
- Documentation covering usage, development workflow, and messaging design.
- CircleCI pipeline for linting, tests, benchmarks, and integration runs.

[Unreleased]: https://github.com/rocketbitz/libfabric-go/compare/v0.0.1...HEAD
[v0.0.1]: https://github.com/rocketbitz/libfabric-go/releases/tag/v0.0.1
