# Messaging API Design (Phase 4)

This document sketches the plan for the first slice of Phase 4 – enabling
message send/receive operations over the existing endpoint + completion queue
infrastructure.

## Goals for the first iteration
- Support basic message send/receive for the sockets provider using active
  endpoints, completion queues, and (optionally) address vectors.
- Maintain correctness without exposing callers to CGO details (no unsafe
  pointer juggling for the public `fi` package).
- Allow both asynchronous posting (user managed completions) and
  higher-level synchronous helpers that block on completions for simple flows.

## Scope and assumptions
- Focus on message/MSG endpoint type (`EndpointTypeMsg`). Tagged, RDM, and data
  message variants will be layered on once the basic pipeline is proven.
- Datagrams, RDM, or provider-specific capabilities are explicitly out of scope
  for the first iteration.
- Memory registration is not required yet; the sockets provider can operate on
  application buffers directly. We will add checks/allocation guards so the API
  is ready for MR-aware providers.

## C API bindings to add (`internal/capi`)
- Thin wrappers for `fi_send`, `fi_recv`, `fi_sendv`, `fi_recvv`, and
  `fi_sendmsg`/`fi_recvmsg` (initially we will exercise the simple `fi_send` and
  `fi_recv` entry points).
- Provider inject limits are fetched via `fi_info` and used to decide when to call `fi_inject`. Buffers are staged through cached context-managed allocations for larger payloads.
- A small `Context` helper that owns a C allocation used as the
  `void *context` pointer passed into libfabric. This avoids violating cgo’s
  “Go pointer to Go pointer” rule. The helper will store a `uintptr` key that
  maps back to Go-managed metadata (see below).

## Public API shape (`fi` package)
- Introduce `type CompletionContext struct` whose lifetime is tied to the Go
  runtime. Construction will register the context inside a global sync.Map keyed
  by `uintptr`. The map entry will carry user metadata (e.g. channel, callback).
  - `CompletionContext.Handle()` will expose the underlying unsafe pointer for
    posting operations (backed by C malloc). Callers choose between reusing a
    context or generating a new one per post.
  - `Release()` removes the context from the map and frees the C allocation.
- Add `type SendRequest` / `RecvRequest` structs to package up buffers, optional
  descriptors, destination/source addresses, flags, and completion context.
  (Flags will map to libfabric message flags; we start with `uint64` passthrough
  until a richer enum set is needed.)
- Extend `*Endpoint` with:
  - `PostSend(req *SendRequest) error`
  - `PostRecv(req *RecvRequest) error`
  - `SendSync(buf []byte, dest Address, cq *CompletionQueue, timeout time.Duration) error`
    which posts a send and loops on `cq.ReadContext()` (or `WaitSet`) until the
    matching context completes. Sync helpers are optional but provide a
    developer-friendly sanity check path.
- Add sentinel errors (`ErrContextExpired`, etc.) so queue consumers can notice
  when a completion arrives for an unknown key (e.g. due to double-release).

## Completion processing
- `CompletionQueue.ReadContext()` already returns the context pointer. We will
  add helper `ResolveContext(ctx unsafe.Pointer) (*CompletionContext, bool)` to
  map completions back to user data.
- Tests will validate posting multiple sends/recvs with unique contexts and
  ensure they surface via CQ reads on the sockets provider.

## Testing strategy
1. Unit tests for context manager: allocation, lookup, release, double release.
2. Integration-style tests (behind `sockets` availability check) that:
   - Open endpoint + CQ/AV.
   - Post a recv, then send to self via AV address, ensuring payload round-trip.
   - Exercise `SendSync` convenience helper to prove the blocking path.

## Follow-up work (not in first slice)
- Tagged messaging (tsend/trecv) and RDM flows.
- Provider feature detection (`fiRXAttr.caps & FI_MSG`) and flag negotiation.
- Memory registration integration + descriptor management.
- Higher-level convenience wrappers that marshal `[]byte` into pooled buffers.

With this plan in place we can start implementing the capi bindings, context
manager, and high-level endpoint helpers in subsequent steps.

## Upcoming Work: Tagged Messaging & RDM Support
- Detect provider capabilities via `fi_info` caps/mode bits (`FI_TAGGED`, `FI_RDM`, `FI_MULTI_RECV`, etc.) and expose helpers on `fi.Info` / `fi.Descriptor`.
- Expose capability metadata on `fi.Info`/`fi.Descriptor`, endpoint inject limits, and tagged verbs. Tagged send/receive helpers (`PostTaggedSend`, `PostTaggedRecv`) now wrap the new capi wrappers with capability checks, and completions expose tag/data metadata (see `TaggedRecvSync`). Next: broaden coverage to RDM flows and provider-specific capabilities, and add examples leveraging tagged completions.
- Add negotiation helpers to fall back gracefully when providers do not advertise the required caps.
- Reuse the completion context plumbing implemented for message queues to surface tag/data fields in completions.
- Plan RDM data-path tests by spinning up a pair of endpoints with compatible providers (e.g. sockets RDM) and exercising tagged round-trips.

## Provider Selection
- Discovery helpers expose capability bits (`Info.SupportsTagged`, `SupportsRDM`, `SupportsDatagram`) and endpoint types.
- Tests/examples should call `Discovery.SupportsEndpointType` before attempting to open endpoints to avoid crashing providers that lack a feature.
- RDM messaging reuses the base message helpers once an `EndpointTypeRDM` descriptor is chosen; tagged flows depend on `CQFormatTagged`.

- Example: `examples/msg_basic` demonstrates provider selection, endpoint bring-up, and send/recv flow.
- New example: `examples/provider_switch` negotiates between RDM- and MSG-capable descriptors, binding address vectors when available and falling back to the basic MSG helper pipeline.

## Provider Switching Playbook
- Prefer RDM endpoints when `Info.SupportsRDM()` returns true; they reuse the same send/recv helpers but require address vector setup for peer addressing.
- Bind a shared `AddressVector` to each endpoint before enabling when working with RDM or datagram providers; `(*Endpoint).RegisterAddress` wraps the common `Name()` + `InsertRaw` sequence so examples stay concise.
- Fall back to MSG endpoints when RDM capabilities are absent. The synchronous helpers (`SendSync`, `RecvSync`) operate without explicit addressing because connection-oriented MSG endpoints default to `AddressUnspecified`.
- Surface which provider path is active so operators understand which network requirements and semantics apply (reliability guarantees, addressing model, inject limits).

### Synchronous Helper Cookbook
```go
cq, _ := domain.OpenCompletionQueue(nil)
ep, _ := desc.OpenEndpoint(domain)
_ = ep.BindCompletionQueue(cq, fi.BindSend|fi.BindRecv)
_ = ep.Enable()

recvBuf := make([]byte, len(payload))
if err := ep.RecvSync(recvBuf, cq, time.Second); err != nil {
	// Handle ErrTimeout (no completion before deadline) or provider errors.
}
ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
defer cancel()
if err := ep.SendSyncContext(ctx, payload, peerAddr, cq, -1); err != nil {
	// Context cancellation surfaces as context.DeadlineExceeded or context.Canceled.
}
```
- Use non-negative timeouts when multiplexing multiple contexts so callers can
  detect stalled operations. For datagram/RDM flows, populate `peerAddr` via an
  address vector lookup.
- Tagged helpers (`TaggedSendSync`/`TaggedRecvSync`) rely on the same polling
  loop but return the provider's tag/data fields for additional routing.
- Context-based variants (`SendSyncContext`, `RecvSyncContext`, and their
  tagged equivalents) accept a `context.Context` alongside the timeout so tests
  and tooling can cancel blocked waits without relying on deadlines alone; pass
  `-1` for the timeout to rely exclusively on the context when appropriate.

### Datagram Test Configuration
- Enable additional providers in integration tests with
  `LIBFABRIC_TEST_DGRAM_PROVIDERS=efa,sockets`.
- Provide provider-specific queue/address hints via
  `LIBFABRIC_TEST_DGRAM_HINTS=efa:cq_wait=fd,cq_format=data` (keys support
  `cq_format`, `cq_wait`, `cq_size`, `av_type`, `av_count`).
- Unknown providers continue to default to sockets-only coverage to avoid
  undefined provider behaviour until custom hints are supplied.

## Phase 4 Wrap-up Prep
- Document datagram flow requirements: describe the need for per-packet addressing, lack of reliability guarantees, and how to extend the address-vector handshake for `EndpointTypeDgram`.
- Expand synchronous helper guidance so developers know when to rely on `SendSync`/`RecvSync` versus manual completion polling; call out timeout behavior and strategies for multiplexing completions.
- Track remaining polish items (provider hint recipes, additional examples) in the Phase 4 notes within `ROADMAP.md` so follow-up work stays visible.
