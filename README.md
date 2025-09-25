# libfabric-go

`libfabric-go` provides Go bindings for the [libfabric](https://ofiwg.github.io/libfabric/) high-performance fabric interface. The library exposes both low-level APIs that closely mirror the upstream C interface and ergonomic, idiomatic abstractions for building Go networking applications on top of libfabric providers.

## Project Status
- Phase 0 (repository scaffolding & tooling) is in progress.
- See [`ROADMAP.md`](ROADMAP.md) for the complete implementation plan and milestone checklist.

## Getting Started
1. Ensure Go 1.22+ is installed with CGO enabled.
2. Install libfabric development headers and libraries. On macOS with Homebrew:
   ```bash
   brew install libfabric pkg-config
   ```
   On Linux (Debian/Ubuntu):
   ```bash
   sudo apt-get install libfabric-dev pkg-config
   ```
3. Install [just](https://github.com/casey/just) for local task automation. On macOS:
   ```bash
   brew install just
   ```
   On Linux (Debian/Ubuntu):
   ```bash
   sudo apt-get install just
   ```
4. Clone the repository and verify the toolchain:
   ```bash
   git clone https://github.com/rocketbitz/libfabric-go.git
   cd libfabric-go
   just build
   ```

## Repository Layout (planned)
- `internal/capi`: thin CGO layer, enum/constant mappings, unsafe helpers.
- `fi`: low-level Go wrappers for libfabric resources (fabric, domains, endpoints, queues).
- `transport`: optional higher-level ergonomics built atop the low-level bindings.
- `examples/`: runnable usage examples.
- `integration/`: provider-specific integration tests.

## Contributing
- Run `just check` before submitting changes.
- Document provider assumptions and threading behavior in code comments and docs.
- File issues for missing API coverage or provider-specific bugs so they can be prioritized on the roadmap.

## License
TBD (add once decided).
