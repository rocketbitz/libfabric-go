package fi

import (
	"errors"
	"time"
	"unsafe"

	"github.com/rocketbitz/libfabric-go/internal/capi"
)

// PassiveEndpoint wraps a libfabric passive endpoint handle.
type PassiveEndpoint struct {
	handle *capi.PassiveEndpoint
}

// OpenPassiveEndpoint opens a passive endpoint for the descriptor.
func (d Descriptor) OpenPassiveEndpoint(fabric *Fabric) (*PassiveEndpoint, error) {
	if fabric == nil || fabric.handle == nil {
		return nil, errors.New("libfabric: fabric required for passive endpoint")
	}
	pep, err := capi.OpenPassiveEndpoint(fabric.handle, d.entry)
	if err != nil {
		return nil, err
	}
	return &PassiveEndpoint{handle: pep}, nil
}

// Close releases the underlying passive endpoint handle.
func (p *PassiveEndpoint) Close() error {
	if p == nil || p.handle == nil {
		return nil
	}
	err := p.handle.Close()
	p.handle = nil
	return err
}

// BindEventQueue binds the passive endpoint to an event queue.
func (p *PassiveEndpoint) BindEventQueue(eq *EventQueue, flags BindFlag) error {
	if p == nil || p.handle == nil {
		return errors.New("libfabric: passive endpoint closed")
	}
	if eq == nil || eq.handle == nil {
		return errors.New("libfabric: event queue required")
	}
	return p.handle.BindEventQueue(eq.handle, uint64(flags))
}

// Listen transitions the passive endpoint into a listening state.
func (p *PassiveEndpoint) Listen() error {
	if p == nil || p.handle == nil {
		return errors.New("libfabric: passive endpoint closed")
	}
	return p.handle.Listen()
}

// Name returns the provider-specific address for the passive endpoint.
func (p *PassiveEndpoint) Name() ([]byte, error) {
	if p == nil || p.handle == nil {
		return nil, errors.New("libfabric: passive endpoint closed")
	}
	return p.handle.Name()
}

// ConnectionEventType enumerates connection-management events.
type ConnectionEventType = capi.CMEventType

const (
	// ConnectionEventConnReq indicates a new connection request is pending.
	ConnectionEventConnReq = ConnectionEventType(capi.CMEventConnReq)
	// ConnectionEventConnected indicates a connection has been established.
	ConnectionEventConnected = ConnectionEventType(capi.CMEventConnected)
	// ConnectionEventShutdown indicates the connection has been shut down.
	ConnectionEventShutdown = ConnectionEventType(capi.CMEventShutdown)
)

// ConnectionEvent represents a connection-management event.
type ConnectionEvent struct {
	cm *capi.CMEvent
}

// Type returns the event type.
func (e *ConnectionEvent) Type() ConnectionEventType {
	if e == nil || e.cm == nil {
		return 0
	}
	return ConnectionEventType(e.cm.Event)
}

// FID returns the associated fid pointer for the event.
func (e *ConnectionEvent) FID() unsafe.Pointer {
	if e == nil || e.cm == nil {
		return nil
	}
	return e.cm.FID
}

// Data returns the optional provider-supplied connection payload.
func (e *ConnectionEvent) Data() []byte {
	if e == nil || e.cm == nil {
		return nil
	}
	return append([]byte(nil), e.cm.Data...)
}

// Errno returns the provider error code attached to the event, if any. For
// CM events this is typically reported via dedicated error events, so the
// value is zero for successful notifications.
func (e *ConnectionEvent) Errno() Errno {
	return 0
}

// ProviderErr returns the provider-specific errno value. CM success events do
// not carry provider errno information, so this is zero in the common case.
func (e *ConnectionEvent) ProviderErr() int {
	return 0
}

// Info produces a Go snapshot of the fi_info attached to the event.
func (e *ConnectionEvent) Info() Info {
	if e == nil || e.cm == nil {
		return Info{}
	}
	return infoFromEntry(e.cm.Info)
}

// OpenEndpoint creates an active endpoint using the event's connection info.
func (e *ConnectionEvent) OpenEndpoint(domain *Domain) (*Endpoint, error) {
	if e == nil || e.cm == nil {
		return nil, errors.New("libfabric: nil connection event")
	}
	return domain.OpenEndpointWithInfo(e.cm.Info)
}

// Free releases the fi_info associated with the event.
func (e *ConnectionEvent) Free() {
	if e == nil || e.cm == nil {
		return
	}
	e.cm.FreeInfo()
	e.cm = nil
}

// ReadCM reads a connection-management event from the event queue.
func (e *EventQueue) ReadCM(timeout time.Duration) (*ConnectionEvent, error) {
	if e == nil || e.handle == nil {
		return nil, ErrInvalidHandle{"event queue"}
	}
	cm, err := e.handle.ReadCM(timeout)
	if err != nil {
		return nil, translateErr(err, ErrNoEvent)
	}
	if cm == nil {
		return nil, ErrNoEvent
	}
	return &ConnectionEvent{cm: cm}, nil
}
