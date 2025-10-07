# Usage Guide

This document outlines how to configure libfabric providers when working with
`libfabric-go`, highlights the primary packages (`fi` and `client`), and links to
runnable examples.

## Provider Configuration

Most examples and tests default to the sockets provider. Override providers via
environment variables:

- `FI_PROVIDER` – libfabric provider name (e.g. `sockets`, `efa`).
- `FI_SOCKETS_IFACE` – network interface for the sockets provider (`lo` on
  Linux, `lo0` on macOS).
- `LIBFABRIC_EXAMPLE_PROVIDER` – provider used by programs under `examples/`.
- `LIBFABRIC_TEST_*` – opt-in providers for integration tests (see below).

### Integration Test Environment

| Variable | Purpose |
| --- | --- |
| `LIBFABRIC_TEST_TAGGED` | Enable tagged messaging tests. |
| `LIBFABRIC_TEST_DGRAM_PROVIDERS` | Comma-separated datagram providers. |
| `LIBFABRIC_TEST_RMA_PROVIDERS` | Providers with RMA capabilities. |
| `LIBFABRIC_TEST_CLIENT_RDM_PROVIDERS` | Reliable datagram providers for the high-level client tests. |
| `LIBFABRIC_TEST_CLIENT_RDM_HINTS` | Provider-specific hints for client RDM tests (`provider:key=value`). |
| `LIBFABRIC_TEST_CLIENT_MSG_PROVIDERS` | MSG providers for the high-level client tests. |
| `LIBFABRIC_TEST_CLIENT_MSG_HINTS` | Provider hints for MSG tests. |

Set the corresponding `*_HINTS` variables to tune node/service/interface
selection when targeting remote hardware:

```bash
export LIBFABRIC_TEST_CLIENT_MSG_PROVIDERS=efa,sockets
export LIBFABRIC_TEST_CLIENT_MSG_HINTS="efa:node=10.0.0.10,service=7500; sockets:node=127.0.0.1"
```

### Examples

Run the example binaries via `go run` or `just integration`:

```bash
LIBFABRIC_EXAMPLE_PROVIDER=sockets FI_SOCKETS_IFACE=lo0 go run ./examples/msg_basic
```

The integration suite wraps the examples and skips unsupported combinations when
capabilities are missing.

## Package Overview

### `fi`

Low-level bindings for fabrics, domains, endpoints, queues, memory registration,
RMA, AVs, and wait sets. Key entry points:

- `fi.DiscoverDescriptors` – provider discovery with optional hints.
- `fi.Domain.RegisterMemory` / `RegisterMemoryWithOptions` – register buffers.
- `fi.Endpoint.SendSync` / `ReceiveSync` – blocking messaging helpers.
- `fi.RMARequest` and `Endpoint.PostRead` / `PostWrite` – RMA flows.

See `examples/rma_basic` and the unit tests under `fi/` for usage patterns.

### `client`

High-level abstraction that manages discovery, endpoint creation, completion
queues, peer registration, tracing, and metrics. Highlights:

- `client.Dial` / `client.Connect` for connecting MSG and RDM endpoints.
- `client.Listen` + `Accept` for MSG listeners.
- Asynchronous handlers via `RegisterSendHandler` and `RegisterReceiveHandler`.
- Metrics hooks for Prometheus and OpenTelemetry (`MetricHook`).

`examples/client_basic` demonstrates the synchronous API; the unit tests in
`client/client_test.go` cover asynchronous flows and environment configuration.

## Memory Registration Tips

- Check provider capabilities through `fi.Info.SupportsRemoteRead`,
  `SupportsRemoteWrite`, and `RequiresMRMode` to infer required flags.
- Use `fi.MRPool` for fixed-size buffer reuse to avoid repeated registrations.
- When providers demand `FI_MR_LOCAL`, the client layer automatically registers
  temporary buffers before issuing operations.

## Troubleshooting

- `fi_enable: Operation not permitted` – the selected provider is unsupported in
  the current environment; fall back to `FI_PROVIDER=sockets` to run the examples.
- `no descriptor succeeded` – discovery found providers but none satisfied the
  requested capabilities. Review the `LIBFABRIC_*` environment variables and
  match the example to a provider that advertises the necessary features.

For additional details and design background, see
[`docs/MESSAGING_DESIGN.md`](MESSAGING_DESIGN.md).
