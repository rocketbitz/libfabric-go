# libfabric-go

`libfabric-go` provides Go bindings for the [libfabric](https://ofiwg.github.io/libfabric/) high-performance fabric interface. The library exposes both low-level APIs that closely mirror the upstream C interface and ergonomic, idiomatic abstractions for building Go networking applications on top of libfabric providers.

## Project Status
- Phases 0–4 are complete (baseline scaffolding, discovery, queueing, messaging).
- Phase 5 (memory registration & RMA) is in progress; see [`ROADMAP.md`](ROADMAP.md)
  and [`docs/PHASE5_PLAN.md`](docs/PHASE5_PLAN.md) for current milestones.
- Integration tests for tagged messaging and future RMA/RDM coverage are gated by
  environment variables (e.g., `LIBFABRIC_TEST_TAGGED=1`,
  `LIBFABRIC_TEST_DGRAM_PROVIDERS=sockets`,
  `LIBFABRIC_TEST_RMA_PROVIDERS=<provider>`).
- Discovery helpers now surface provider MR requirements and remote read/write
  capabilities so domains can enforce mandatory flags automatically.
- `fi.MRPool` provides a reusable pool of registered buffers to reduce
  registration churn when cycling through similarly sized transfers.

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

## Examples
- `examples/msg_basic`: minimal MSG endpoint send/recv on the sockets provider.
- `examples/rma_basic`: loopback RMA write/read using memory registration, address vectors, and the sockets RDM endpoint (defaults to the `sockets` provider; override via `LIBFABRIC_EXAMPLE_PROVIDER=<provider>`). The example reports a clear error when the discovered descriptor lacks `FI_REMOTE_READ`/`FI_REMOTE_WRITE` support.
- Memory helpers now include `Domain.RegisterMemoryWithOptions`,
  `Domain.RegisterMemoryPointer`, and `fi.MRPool` for advanced RMA flows.

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
Distributed under the MIT License. See [`LICENSE`](LICENSE) for details.
