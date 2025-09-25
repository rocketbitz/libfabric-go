# Contributing to libfabric-go

Thank you for your interest in contributing! This document captures the initial workflow while the project is under active development.

## Development Workflow
1. Fork the repository and create a topic branch for your change.
2. Ensure libfabric headers/libraries are installed locally and CGO is enabled.
3. Run formatting, linting, and tests with `just check`.
4. Keep commits focused; include tests or examples when adding feature coverage.
5. Update documentation and the roadmap checklist as milestones are delivered.

## Coding Guidelines
- Favor clear, idiomatic Go while preserving libfabric terminology where it aids discoverability.
- Wrap CGO calls inside `internal/capi`; only expose safe Go types from public packages.
- Document ownership semantics and threading expectations for every exported API.
- Minimize allocations in hot paths; include benchmarks when optimizing performance-sensitive code.

## Reporting Issues
- Include libfabric version, provider details, and reproduction steps.
- Note whether the issue occurs with specific providers or configurations.
- Label issues according to the roadmap phase when possible.

We welcome early feedback and contributions while the bindings are being built out. Thanks for helping make libfabric-go reliable and ergonomic!
