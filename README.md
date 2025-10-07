# libfabric-go

`libfabric-go` provides Go bindings for the
[libfabric](https://ofiwg.github.io/libfabric/) high-performance fabric
interface. It exposes:

- **Low-level primitives** under the `fi` package that closely mirror the C API
  while offering safer resource management, automatic error translation, and
  utilities such as memory-registration pools.
- **High-level ergonomics** under the `client` package for dialing providers,
  dispatching asynchronous send/receive handlers, recording metrics, and wiring
  tracing/logging integrations.

This repository is ready for its first tagged release (`v0.0.1`). The
[`ROADMAP`](ROADMAP.md) tracks follow-on work beyond this milestone.

## Requirements

- Go 1.22 or newer with CGO enabled.
- libfabric development headers and shared libraries discoverable via
  `pkg-config`.
- A provider supported by your platform (the sockets provider is exercised by
  the examples and CI).

Install libfabric and helper tooling:

```bash
# macOS (Homebrew)
brew install libfabric pkg-config just

# Debian / Ubuntu
sudo apt-get update
sudo apt-get install -y libfabric-dev pkg-config just
```

## Quick Start

Add the module and run a basic client loopback:

```bash
go get github.com/rocketbitz/libfabric-go@latest
```

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/rocketbitz/libfabric-go/client"
    fi "github.com/rocketbitz/libfabric-go/fi"
)

func main() {
    cfg := client.Config{
        Provider:     "sockets",
        EndpointType: fi.EndpointTypeRDM,
        Timeout:      2 * time.Second,
    }

    c, err := client.Dial(cfg)
    if err != nil {
        log.Fatalf("dial: %v", err)
    }
    defer c.Close()

    addr, err := c.LocalAddress()
    if err != nil {
        log.Fatalf("local address: %v", err)
    }
    dest, err := c.RegisterPeer(addr, false)
    if err != nil {
        log.Fatalf("register peer: %v", err)
    }

    if err := c.SendTo(context.Background(), dest, []byte("hello")); err != nil {
        log.Fatalf("send: %v", err)
    }

    buf := make([]byte, 16)
    n, _, err := c.ReceiveFrom(context.Background(), buf)
    if err != nil {
        log.Fatalf("receive: %v", err)
    }
    log.Printf("received %q", string(buf[:n]))
}
```

See [`docs/USAGE.md`](docs/USAGE.md) for a deeper walkthrough covering
discovery, messaging, and memory registration flows.

## Examples

The repository ships runnable programs under `examples/` and matching
integration tests under `integration/`:

- `examples/msg_basic`: minimal MSG endpoint send/receive helper.
- `examples/rma_basic`: demonstrates memory registration and RMA loopback.
- `examples/client_basic`: exercises the high-level client API.

Run them locally with the sockets provider:

```bash
LIBFABRIC_EXAMPLE_PROVIDER=sockets FI_SOCKETS_IFACE=lo just integration
```

## Documentation

- [`docs/USAGE.md`](docs/USAGE.md) – operational guidance, provider knobs, and
  walkthroughs for discovery, messaging, and RMA flows.
- [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) – project structure, testing
  strategy, and release workflow for contributors.
- [`docs/MESSAGING_DESIGN.md`](docs/MESSAGING_DESIGN.md) – design notes behind
  the high-level messaging abstractions.

## Testing

- `just check` runs formatting, linting, and unit tests (race + coverage).
- `just integration` executes the integration suite; additional providers can be
  configured through the `LIBFABRIC_TEST_*` environment variables documented in
  `docs/USAGE.md`.

## Contributing

- File issues for missing API coverage, provider regressions, or documentation
  gaps.
- Discuss substantial design changes in `docs/` before opening a PR when
  possible.
- Ensure new code includes GoDoc comments for exported identifiers and runnable
  examples when applicable.

## License

Distributed under the MIT License. See [`LICENSE`](LICENSE) for details.
