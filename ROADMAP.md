# libfabric-go Implementation Roadmap

## Vision
- Provide a complete CGO-backed Go binding for the libfabric user-space networking stack.
- Expose both low-level (close to C API) and ergonomic high-level Go APIs for fabrics, endpoints, queues, memory registration, and transfers.
- Maintain correctness, performance, and resource safety while keeping the package idiomatic for other Go projects.

## Prerequisites & Environment
- Target Go version >= 1.22 with CGO enabled; document any additional build tags.
- Ensure libfabric development headers and shared libraries are installed and discoverable via `pkg-config`.
- Define supported platforms/providers (e.g. sockets, verbs, EFA) and note any special setup required for integration tests and cloud providers.

## Repository & Module Layout
- Establish root module `github.com/rocketbitz/libfabric-go` with standard Go tooling (linting, gofmt, go vet).
- Plan packages:
  - `internal/capi`: thin CGO layer, constants, error codes, unsafe helpers.
  - `fi`: user-facing low-level wrappers aligned with libfabric concepts (`Fabric`, `Domain`, `Endpoint`, etc.).
  - `fi/memory`, `fi/cq`, `fi/eq`, `fi/av`, etc. for cohesive sub-domains, or use sub-packages within `fi` as needed.
  - `transport`: higher-level convenience interfaces (optional) built atop low-level bindings.
- Add examples and integration tests under `examples/` and `integration/` directories.

## Implementation Phases

### Phase 0 – Baseline Project Setup
- **Complete.** go.mod targets Go 1.22, README/CONTRIBUTING written, and repo hygiene (.gitignore) in place.
- **Complete.** Local tooling configured via `Justfile`, `.golangci.yml`, and helper targets for fmt/lint/test.
- **Complete.** Basic CGO compilation validated through libfabric version query and unit tests.

### Phase 1 – CGO Foundation & Error Handling
- **Complete.** `internal/capi` exposes libfabric headers, error mappings, version helpers, and Go/C conversions.
- **Complete.** Negative status handling, strerror wrappers, and version compatibility guards implemented with tests.

### Phase 2 – Core Discovery & Resource Lifecycle
- **Complete.** Discovery pipeline with option-based hints, fabric/domain wrappers, and sockets-backed tests.
- **Complete (active endpoints).** Endpoint, CQ, EQ scaffolding added with enable/bind flows; passive endpoints TBD.
- Document threading/context semantics alongside future passive endpoint work.

### Phase 3 – Queueing & Addressing Primitives
- **Complete.** Address vectors, CQ/EQ lifecycle, polling helpers, and wait-set support implemented with sockets coverage.
- Continue adding richer examples once messaging APIs are available.

### Phase 4 – Data Transfer Operations
- **Complete.** Base message send/receive posting helpers implemented with inject optimization, completion contexts, and context-aware synchronous wrappers.
- **Complete.** Tagged message helpers validated with RDM/Datagram flows; datagram coverage is tunable via env-based provider hints for future expansion.
- Future polish: capture provider-specific CQ/AV recipes so additional providers (e.g., EFA) can be exercised automatically in CI without bespoke setup.
- Document ordering, error propagation, and buffer lifetime requirements for each operation; consider synchronous helpers that wrap wait sets or contexts.

### Phase 5 – Memory Registration & RMA/Atomic APIs
- Wrap memory registration (`fi_mr_reg`, `fi_mr_bind`, `fi_close`) with support for key management and access flags.
- Implement Remote Memory Access operations (`fi_read`, `fi_write`, `fi_atomic*`) and completion semantics.
- Provide helpers for scatter/gather lists and `iovec` interoperability.
- Add sample benchmark or integration test demonstrating an RMA flow on the sockets provider.

### Phase 6 – High-Level Go Abstractions (Optional Layer)
- Design idiomatic Go interfaces that hide CGO details: e.g. `Client`, `EndpointPair`, buffer pools.
- Implement resource managers that translate contexts into Go structs with automatic cleanup (using `runtime.SetFinalizer` sparingly).
- Offer ergonomic APIs for asynchronous operations (channels, contexts, or callback adapters) while allowing advanced users to drop down to low-level bindings.

### Phase 7 – Testing, QA, and CI
- Unit test conversion utilities and error handling with table-driven tests.
- Create integration tests that spin up local providers (e.g. sockets) to validate end-to-end messaging and RMA flows; gate them behind build tags when providers unavailable.
- Add benchmarks (Go `testing.B`) for critical paths to track performance regressions.
- Wire up CI matrix that installs libfabric (or uses container) and runs lint, unit tests, integration tests (provider permitting), and benchmarks (optionally).

### Phase 8 – Documentation & Examples
- Generate GoDoc comments for all exported symbols; maintain package-level overview with usage notes and caveats.
- Provide runnable examples showing fabric discovery, endpoint setup, message exchange, and memory registration.
- Produce developer docs covering project structure, testing strategy, and provider-specific guidance.

## Milestone Checklist
- [x] Phase 0: Repository scaffolding & tooling complete.
- [x] Phase 1: CGO foundation merged with error helpers and version gating.
- [x] Phase 2: Fabric/domain lifecycle wrappers with tests (passive endpoint coverage pending).
- [x] Phase 3: Queueing primitives (AV, CQ/EQ, wait sets) validated with tests.
- [x] Phase 4: Messaging APIs functioning with integration coverage.
- [ ] Phase 5: Memory registration, RMA, and atomic operations implemented.
- [ ] Phase 6: High-level Go abstractions (optional but planned) stabilized.
- [ ] Phase 7: CI pipeline green across supported providers.
- [ ] Phase 8: Documentation and examples published.

## Outstanding Questions / Research
- Determine which libfabric providers are mandatory for support and how to emulate them in CI.
- Decide how strictly to mirror the C API naming vs. Go idioms (e.g. builder patterns, contexts).
- Evaluate feasibility of code generation for structs/enums to reduce maintenance burden.
- Assess need for thread-safety guarantees or locking in Go wrappers beyond libfabric requirements.
