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

### Phase 5 – Memory Registration & RMA APIs
- Memory registration wrappers now surface provider MR mode/key requirements, auto-enable mandatory flags (e.g. `FI_MR_LOCAL`), and support advanced options via `RegisterMemoryWithOptions` / `RegisterMemoryPointer`.
- Scatter/gather registration is available through `RegisterMemorySegments`, backed by the new `internal/capi.MRIOVec` helper.
- `fi.MRPool` provides reusable registered buffers to cut churn on repeated transfers.
- RMA read/write helpers (`PostRead`/`PostWrite`, sync/context variants) and sockets-based tests exercise the full pipeline; `examples/rma_basic` demonstrates a loopback flow and reports provider capability gaps.
- Follow-up: implement atomic verbs once a provider exposes them and expand descriptor-blob introspection when a backend requires it.

### Phase 6 – High-Level Go Abstractions (Optional Layer)
- **Complete.** `client` package provides discovery, lifecycle management, MR pooling, async completions, source-aware `ReceiveFrom`, handler registration, telemetry hooks (`Logger`, `StructuredLogger`, `Tracer`, `MetricHook`, `Stats`), built-in Prometheus / OpenTelemetry metrics adapters, and MSG listener/connect helpers built atop passive endpoints.
- Follow-ups are tracked below (retry/backpressure helpers, RDM/datagram peer ergonomics) and will inform Phase 7/8 priorities.

### Phase 7 – Testing, QA, and CI
- **Complete.** CI now runs gofmt/lint (golangci-lint), race-enabled unit tests with coverage, and benchmark sweeps via CircleCI. Provider-aware client tests honour `LIBFABRIC_TEST_*` env knobs so sockets remains fast while additional providers (e.g., EFA) can be exercised where hardware is available. The integration harness (`go test -tags=integration ./integration/...`) now includes a dedicated high-level client MSG end-to-end test (`integration/client_e2e_test.go`) alongside the example invocations, keeping table-driven unit tests and dispatcher coverage green under the lint-first workflow. Deferred enhancements (provider-specific lanes, published artifacts, broader provider coverage) are tracked under Outstanding Questions.

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
- [x] Phase 5: Memory registration & RMA helpers implemented (atomic verbs pending provider support).
- [x] Phase 6: High-level Go abstractions (optional but planned) stabilized.
- [x] Phase 7: CI pipeline green across supported providers.
- [ ] Phase 8: Documentation and examples published.

## Outstanding Questions / Research
- Determine which libfabric providers are mandatory for support and how to emulate them in CI.
- Schedule provider-specific CI lanes (e.g., self-hosted AWS EFA) and automate coverage/benchmark artifact publishing once infrastructure is available.
- Extend integration coverage beyond sockets (tagged, RMA, multi-process scenarios) once additional providers are available in CI.
- Decide how strictly to mirror the C API naming vs. Go idioms (e.g. builder patterns, contexts).
- Evaluate feasibility of code generation for structs/enums to reduce maintenance burden.
- Assess need for thread-safety guarantees or locking in Go wrappers beyond libfabric requirements.
- Identify provider coverage for RMA atomic verbs and descriptor blob requirements so the Phase 5 follow-up work can be scheduled.
- Define peer addressing patterns (RDM/datagram) and connection-management primitives for the high-level client layer.
- Explore retry/backpressure and handler orchestration patterns to round out the high-level client ergonomics.
