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

## Roadmap Themes

### Provider Coverage & CI
- Determine which libfabric providers are mandatory for long-term support and
  outline validation requirements for each.
- Stand up provider-specific CI lanes (e.g., self-hosted AWS EFA) and publish
  coverage/benchmark artifacts as part of the pipeline.
- Extend integration coverage beyond sockets, including tagged messaging, RMA,
  and multi-process scenarios once additional providers are available.

### API Evolution & Ergonomics
- Decide how closely the Go surface should mirror the C API versus adopting
  idiomatic builder/context patterns.
- Define peer addressing patterns for RDM/datagram workflows and expand the
  high-level client with connection-management primitives.
- Explore retry, backpressure, and handler orchestration features to round out
  the high-level client.

### Runtime & Code Generation Enhancements
- Evaluate code generation for libfabric structs/enums to reduce maintenance
  overhead.
- Identify provider coverage for RMA atomic verbs and descriptor blobs so the
  remaining low-level gaps can be prioritized.
- Assess thread-safety guarantees and locking strategies required by the Go
  wrappers beyond libfabric's own requirements.
