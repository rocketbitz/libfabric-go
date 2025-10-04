package fi

import (
	"errors"
	"unsafe"

	"github.com/rocketbitz/libfabric-go/internal/capi"
)

// CompletionQueueAttr controls completion queue creation.
type CompletionQueueAttr struct {
	Size            int
	Flags           uint64
	Format          CQFormat
	WaitObj         WaitObj
	SignalingVector int
	WaitCondition   CQWaitCond
}

// EventQueueAttr controls event queue creation.
type EventQueueAttr struct {
	Size            int
	Flags           uint64
	WaitObj         WaitObj
	SignalingVector int
}

// CompletionQueue exposes a completion queue handle.
type CompletionQueue struct {
	handle *capi.CompletionQueue
	format CQFormat
}

// CompletionEvent represents a single completion entry.
type CompletionEvent struct {
	Context unsafe.Pointer
	Tag     uint64
	Data    uint64
	Source  Address
}

// HasTag reports whether the completion carried tag information (tagged CQ format).
func (e *CompletionEvent) HasTag() bool {
	return e != nil && e.Tag != 0
}

// Resolve converts the raw context pointer into a managed CompletionContext and
// executes any completion callbacks registered on that context.
func (e *CompletionEvent) Resolve() (*CompletionContext, error) {
	if e == nil {
		return nil, ErrContextUnknown
	}
	return resolveCompletion(e.Context)
}

// CompletionError contains error details from the provider.
type CompletionError struct {
	Context     unsafe.Pointer
	Err         Errno
	ProviderErr int
	Flags       uint64
	Length      uint64
	Data        uint64
	Tag         uint64
	Buffer      unsafe.Pointer
	ErrData     unsafe.Pointer
	ErrDataSize uint64
	SrcAddr     uint64
}

// Resolve converts the error entry's context pointer into a managed context.
func (e *CompletionError) Resolve() (*CompletionContext, error) {
	if e == nil {
		return nil, ErrContextUnknown
	}
	return resolveCompletion(e.Context)
}

// EventQueue exposes an event queue handle.
type EventQueue struct {
	handle *capi.EventQueue
}

// Event encapsulates an event queue entry.
type Event struct {
	Event   uint32
	FID     unsafe.Pointer
	Context unsafe.Pointer
	Data    uint64
}

// Resolve resolves the event's context pointer, if present.
func (e *Event) Resolve() (*CompletionContext, error) {
	if e == nil || e.Context == nil {
		return nil, ErrContextUnknown
	}
	return resolveCompletion(e.Context)
}

// EventError captures event queue error information.
type EventError struct {
	FID         unsafe.Pointer
	Context     unsafe.Pointer
	Data        uint64
	Err         Errno
	ProviderErr int
	ErrData     unsafe.Pointer
	ErrDataSize uint64
}

// Resolve resolves the event error's context pointer.
func (e *EventError) Resolve() (*CompletionContext, error) {
	if e == nil || e.Context == nil {
		return nil, ErrContextUnknown
	}
	return resolveCompletion(e.Context)
}

// Endpoint wraps a libfabric endpoint handle.
type Endpoint struct {
	handle         *capi.Endpoint
	injectLimit    uintptr
	supportsTagged bool
}

// CQFormat mirrors capi.CQFormat for public use.
type CQFormat = capi.CQFormat

const (
	CQFormatUnspec  = capi.CQFormatUnspec
	CQFormatContext = capi.CQFormatContext
	CQFormatMsg     = capi.CQFormatMsg
	CQFormatData    = capi.CQFormatData
	CQFormatTagged  = capi.CQFormatTagged
)

// WaitObj mirrors capi.WaitObj.
type WaitObj = capi.WaitObj

const (
	WaitNone      = capi.WaitNone
	WaitUnspec    = capi.WaitUnspec
	WaitObjSet    = capi.WaitObjSet
	WaitFD        = capi.WaitFD
	WaitMutexCond = capi.WaitMutexCond
	WaitYield     = capi.WaitYield
	WaitPollFD    = capi.WaitPollFD
)

// CQWaitCond mirrors capi.CQWaitCond.
type CQWaitCond = capi.CQWaitCond

const (
	CQCondNone      = capi.CQCondNone
	CQCondThreshold = capi.CQCondThreshold
)

// BindFlag controls endpoint binding behavior.
type BindFlag uint64

const (
	BindSend BindFlag = BindFlag(capi.BindSend)
	BindRecv BindFlag = BindFlag(capi.BindRecv)
)

// Close releases the completion queue.
func (c *CompletionQueue) Close() error {
	if c == nil || c.handle == nil {
		return nil
	}
	err := c.handle.Close()
	c.handle = nil
	return err
}

// ReadContext retrieves a single completion event if available.
func (c *CompletionQueue) ReadContext() (*CompletionEvent, error) {
	if c == nil || c.handle == nil {
		return nil, ErrInvalidHandle{"completion queue"}
	}
	event, err := c.handle.ReadContext()
	if err != nil {
		return nil, translateErr(err, ErrNoCompletion)
	}
	if event == nil {
		return nil, ErrNoCompletion
	}
	return &CompletionEvent{Context: event.Context, Tag: event.Tag, Data: event.Data, Source: Address(event.SrcAddr)}, nil
}

// ReadError returns the next completion queue error entry if present.
func (c *CompletionQueue) ReadError(flags uint64) (*CompletionError, error) {
	if c == nil || c.handle == nil {
		return nil, ErrInvalidHandle{"completion queue"}
	}
	entry, err := c.handle.ReadError(flags)
	if err != nil {
		return nil, translateErr(err, ErrNoCompletion)
	}
	if entry == nil {
		return nil, ErrNoCompletion
	}
	return &CompletionError{
		Context:     entry.Context,
		Err:         entry.Err,
		ProviderErr: entry.ProviderErr,
		Flags:       entry.Flags,
		Length:      entry.Length,
		Data:        entry.Data,
		Tag:         entry.Tag,
		Buffer:      entry.Buffer,
		ErrData:     entry.ErrData,
		ErrDataSize: entry.ErrDataSize,
		SrcAddr:     entry.SrcAddr,
	}, nil
}

// Close releases the event queue.
func (e *EventQueue) Close() error {
	if e == nil || e.handle == nil {
		return nil
	}
	err := e.handle.Close()
	e.handle = nil
	return err
}

// Read retrieves the next event queue entry.
func (e *EventQueue) Read(flags uint64) (*Event, error) {
	if e == nil || e.handle == nil {
		return nil, ErrInvalidHandle{"event queue"}
	}
	entry, err := e.handle.Read(flags)
	if err != nil {
		return nil, translateErr(err, ErrNoEvent)
	}
	if entry == nil {
		return nil, ErrNoEvent
	}
	return &Event{
		Event:   entry.Event,
		FID:     entry.FID,
		Context: entry.Context,
		Data:    entry.Data,
	}, nil
}

// ReadError retrieves the next event queue error entry.
func (e *EventQueue) ReadError(flags uint64) (*EventError, error) {
	if e == nil || e.handle == nil {
		return nil, ErrInvalidHandle{"event queue"}
	}
	entry, err := e.handle.ReadError(flags)
	if err != nil {
		return nil, translateErr(err, ErrNoEvent)
	}
	if entry == nil {
		return nil, ErrNoEvent
	}
	return &EventError{
		FID:         entry.FID,
		Context:     entry.Context,
		Data:        entry.Data,
		Err:         entry.Err,
		ProviderErr: entry.ProviderErr,
		ErrData:     entry.ErrData,
		ErrDataSize: entry.ErrDataSize,
	}, nil
}

// Close releases the endpoint.
func (e *Endpoint) Close() error {
	if e == nil || e.handle == nil {
		return nil
	}
	err := e.handle.Close()
	e.handle = nil
	return err
}

// OpenCompletionQueue opens a completion queue for the domain.
func (d *Domain) OpenCompletionQueue(attr *CompletionQueueAttr) (*CompletionQueue, error) {
	if d == nil || d.handle == nil {
		return nil, ErrInvalidHandle{"domain"}
	}

	var ca *capi.CQAttr
	var tmp capi.CQAttr
	format := CQFormatUnspec
	if attr != nil {
		tmp = capi.CQAttr{
			Size:            attr.Size,
			Flags:           attr.Flags,
			Format:          capi.CQFormat(attr.Format),
			WaitObj:         capi.WaitObj(attr.WaitObj),
			SignalingVector: attr.SignalingVector,
			WaitCondition:   capi.CQWaitCond(attr.WaitCondition),
		}
		ca = &tmp
		format = attr.Format
	}

	handle, err := capi.OpenCompletionQueue(d.handle, ca)
	if err != nil {
		return nil, err
	}
	return &CompletionQueue{handle: handle, format: format}, nil
}

// OpenEventQueue opens an event queue on the fabric.
func (f *Fabric) OpenEventQueue(attr *EventQueueAttr) (*EventQueue, error) {
	if f == nil || f.handle == nil {
		return nil, ErrInvalidHandle{"fabric"}
	}

	var ea *capi.EQAttr
	var tmp capi.EQAttr
	if attr != nil {
		tmp = capi.EQAttr{
			Size:            attr.Size,
			Flags:           attr.Flags,
			WaitObj:         capi.WaitObj(attr.WaitObj),
			SignalingVector: attr.SignalingVector,
		}
		ea = &tmp
	}

	handle, err := capi.OpenEventQueue(f.handle, ea)
	if err != nil {
		return nil, err
	}
	return &EventQueue{handle: handle}, nil
}

// ErrInvalidHandle indicates a nil or closed handle was used.
type ErrInvalidHandle struct {
	Resource string
}

func (e ErrInvalidHandle) Error() string {
	return "invalid or closed " + e.Resource + " handle"
}

// OpenEndpoint opens an endpoint using the descriptor information.
func (d Descriptor) OpenEndpoint(domain *Domain) (*Endpoint, error) {
	if domain == nil || domain.handle == nil {
		return nil, ErrInvalidHandle{"domain"}
	}
	ep, err := capi.OpenEndpoint(domain.handle, d.entry)
	if err != nil {
		return nil, err
	}
	return &Endpoint{
		handle:         ep,
		injectLimit:    d.entry.InjectSize(),
		supportsTagged: d.entry.Caps()&capi.CapTagged != 0,
	}, nil
}

// OpenEndpointWithInfo opens an endpoint using connection-management info.
func (d *Domain) OpenEndpointWithInfo(entry capi.InfoEntry) (*Endpoint, error) {
	if d == nil || d.handle == nil {
		return nil, ErrInvalidHandle{"domain"}
	}
	ep, err := capi.OpenEndpointWithInfo(d.handle, entry)
	if err != nil {
		return nil, err
	}
	info := infoFromEntry(entry)
	return &Endpoint{
		handle:         ep,
		injectLimit:    info.InjectSize,
		supportsTagged: info.SupportsTagged(),
	}, nil
}

// BindCompletionQueue binds the endpoint to a completion queue with flags.
func (e *Endpoint) BindCompletionQueue(cq *CompletionQueue, flags BindFlag) error {
	if e == nil || e.handle == nil {
		return ErrInvalidHandle{"endpoint"}
	}
	if cq == nil || cq.handle == nil {
		return ErrInvalidHandle{"completion queue"}
	}
	return e.handle.BindCompletionQueue(cq.handle, uint64(flags))
}

// BindEventQueue binds the endpoint to an event queue with flags.
func (e *Endpoint) BindEventQueue(eq *EventQueue, flags BindFlag) error {
	if e == nil || e.handle == nil {
		return ErrInvalidHandle{"endpoint"}
	}
	if eq == nil || eq.handle == nil {
		return ErrInvalidHandle{"event queue"}
	}
	return e.handle.BindEventQueue(eq.handle, uint64(flags))
}

// BindAddressVector binds the endpoint to the specified address vector.
func (e *Endpoint) BindAddressVector(av *AddressVector, flags BindFlag) error {
	if e == nil || e.handle == nil {
		return ErrInvalidHandle{"endpoint"}
	}
	if av == nil || av.handle == nil {
		return ErrInvalidHandle{"address vector"}
	}
	return e.handle.BindAddressVector(av.handle, uint64(flags))
}

// Accept acknowledges a pending connection request.
func (e *Endpoint) Accept(params []byte) error {
	if e == nil || e.handle == nil {
		return ErrInvalidHandle{"endpoint"}
	}
	var ptr unsafe.Pointer
	var length uintptr
	if len(params) > 0 {
		ptr = unsafe.Pointer(&params[0])
		length = uintptr(len(params))
	}
	return e.handle.Accept(ptr, length)
}

// Connect initiates a connection request for the endpoint.
func (e *Endpoint) Connect(params []byte) error {
	if e == nil || e.handle == nil {
		return ErrInvalidHandle{"endpoint"}
	}
	var ptr unsafe.Pointer
	var length uintptr
	if len(params) > 0 {
		ptr = unsafe.Pointer(&params[0])
		length = uintptr(len(params))
	}
	return e.handle.Connect(ptr, length)
}

// Enable transitions the endpoint into an active state.
func (e *Endpoint) Enable() error {
	if e == nil || e.handle == nil {
		return ErrInvalidHandle{"endpoint"}
	}
	return e.handle.Enable()
}

// Name returns the provider-specific address associated with the endpoint.
func (e *Endpoint) Name() ([]byte, error) {
	if e == nil || e.handle == nil {
		return nil, ErrInvalidHandle{"endpoint"}
	}
	return e.handle.Name()
}

// RegisterAddress resolves the endpoint's address via Name() and inserts it into
// the provided address vector, returning the provider-specific fi_addr_t.
func (e *Endpoint) RegisterAddress(av *AddressVector, flags uint64) (Address, error) {
	if e == nil || e.handle == nil {
		return 0, ErrInvalidHandle{"endpoint"}
	}
	if av == nil || av.handle == nil {
		return 0, ErrInvalidHandle{"address vector"}
	}
	addrBytes, err := e.Name()
	if err != nil {
		return 0, err
	}
	return av.InsertRaw(addrBytes, flags)
}

// InjectLimit reports the provider's reported inject size hint in bytes.
func (e *Endpoint) InjectLimit() uintptr {
	if e == nil {
		return 0
	}
	return e.injectLimit
}

// SupportsTagged indicates whether the endpoint can perform tagged messaging operations.
func (e *Endpoint) SupportsTagged() bool {
	if e == nil {
		return false
	}
	return e.supportsTagged
}

// Pointer exposes the underlying fid_ep pointer.
func (e *Endpoint) Pointer() unsafe.Pointer {
	if e == nil || e.handle == nil {
		return nil
	}
	return e.handle.Pointer()
}

func translateErr(err error, sentinel error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, capi.ErrAgain) {
		return sentinel
	}
	if errors.Is(err, capi.ErrTimedOut) {
		return ErrTimeout
	}
	return err
}
