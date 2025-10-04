//go:build cgo

package capi

import (
	"fmt"
	"time"
	"unsafe"
)

/*
#cgo pkg-config: libfabric
#include <stdlib.h>
#include <rdma/fabric.h>
#include <rdma/fi_cm.h>
#include <rdma/fi_domain.h>
#include <rdma/fi_endpoint.h>
#include <rdma/fi_eq.h>
*/
import "C"

// Endpoint wraps a libfabric fid_ep handle.
type Endpoint struct {
	ptr *C.struct_fid_ep
}

// CompletionQueue wraps a libfabric fid_cq handle.
type CompletionQueue struct {
	ptr    *C.struct_fid_cq
	format C.enum_fi_cq_format
}

// EventQueue wraps a libfabric fid_eq handle.
type EventQueue struct {
	ptr *C.struct_fid_eq
}

// PassiveEndpoint wraps a libfabric fid_pep handle.
type PassiveEndpoint struct {
	ptr *C.struct_fid_pep
}

// CQError captures details from fi_cq_readerr.
type CQError struct {
	Context     unsafe.Pointer
	Flags       uint64
	Length      uint64
	Buffer      unsafe.Pointer
	Data        uint64
	Tag         uint64
	Err         Errno
	ProviderErr int
	ErrData     unsafe.Pointer
	ErrDataSize uint64
	SrcAddr     uint64
}

// CQEvent represents a single completion queue entry.
type CQEvent struct {
	Context unsafe.Pointer
	Tag     uint64
	Data    uint64
	SrcAddr uint64
}

// CQFormat mirrors enum fi_cq_format.
type CQFormat int

const (
	CQFormatUnspec  CQFormat = CQFormat(C.FI_CQ_FORMAT_UNSPEC)
	CQFormatContext CQFormat = CQFormat(C.FI_CQ_FORMAT_CONTEXT)
	CQFormatMsg     CQFormat = CQFormat(C.FI_CQ_FORMAT_MSG)
	CQFormatData    CQFormat = CQFormat(C.FI_CQ_FORMAT_DATA)
	CQFormatTagged  CQFormat = CQFormat(C.FI_CQ_FORMAT_TAGGED)
)

// WaitObj mirrors enum fi_wait_obj.
type WaitObj int

const (
	WaitNone      WaitObj = WaitObj(C.FI_WAIT_NONE)
	WaitUnspec    WaitObj = WaitObj(C.FI_WAIT_UNSPEC)
	WaitObjSet    WaitObj = WaitObj(C.FI_WAIT_SET)
	WaitFD        WaitObj = WaitObj(C.FI_WAIT_FD)
	WaitMutexCond WaitObj = WaitObj(C.FI_WAIT_MUTEX_COND)
	WaitYield     WaitObj = WaitObj(C.FI_WAIT_YIELD)
	WaitPollFD    WaitObj = WaitObj(C.FI_WAIT_POLLFD)
)

// CQWaitCond mirrors enum fi_cq_wait_cond.
type CQWaitCond int

const (
	CQCondNone      CQWaitCond = CQWaitCond(C.FI_CQ_COND_NONE)
	CQCondThreshold CQWaitCond = CQWaitCond(C.FI_CQ_COND_THRESHOLD)
)

// CQAttr configures fi_cq_open.
type CQAttr struct {
	Size            int
	Flags           uint64
	Format          CQFormat
	WaitObj         WaitObj
	SignalingVector int
	WaitCondition   CQWaitCond
}

// EQAttr configures fi_eq_open.
type EQAttr struct {
	Size            int
	Flags           uint64
	WaitObj         WaitObj
	SignalingVector int
}

const (
	BindSend = uint64(C.FI_SEND)
	BindRecv = uint64(C.FI_RECV)
)

// EQEvent represents an event queue entry.
type EQEvent struct {
	Event   uint32
	FID     unsafe.Pointer
	Context unsafe.Pointer
	Data    uint64
}

// EQError captures error details from fi_eq_readerr.
type EQError struct {
	FID         unsafe.Pointer
	Context     unsafe.Pointer
	Data        uint64
	Err         Errno
	ProviderErr int
	ErrData     unsafe.Pointer
	ErrDataSize uint64
}

// CMEventType represents connection management events reported on event queues.
type CMEventType uint32

const (
	CMEventConnReq   CMEventType = CMEventType(C.FI_CONNREQ)
	CMEventConnected CMEventType = CMEventType(C.FI_CONNECTED)
	CMEventShutdown  CMEventType = CMEventType(C.FI_SHUTDOWN)
)

// CMEvent captures fi_eq_cm_entry data.
type CMEvent struct {
	Event    CMEventType
	FID      unsafe.Pointer
	Info     InfoEntry
	Data     []byte
	ownsInfo bool
}

// OpenEndpoint creates an active endpoint from the supplied domain and
// fi_info descriptor.
func OpenEndpoint(domain *Domain, entry InfoEntry) (*Endpoint, error) {
	if domain == nil || domain.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_endpoint")
	}
	if entry.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_endpoint")
	}

	var ep *C.struct_fid_ep
	status := C.fi_endpoint(domain.ptr, entry.ptr, &ep, nil)
	if err := ErrorFromStatus(int(status), "fi_endpoint"); err != nil {
		return nil, err
	}
	return &Endpoint{ptr: ep}, nil
}

// Close releases the endpoint.
func (e *Endpoint) Close() error {
	if e == nil || e.ptr == nil {
		return nil
	}
	status := C.fi_close((*C.struct_fid)(unsafe.Pointer(e.ptr)))
	if err := ErrorFromStatus(int(status), "fi_close(endpoint)"); err != nil {
		return err
	}
	e.ptr = nil
	return nil
}

// OpenCompletionQueue opens a completion queue on the provided domain.
func OpenCompletionQueue(domain *Domain, attr *CQAttr) (*CompletionQueue, error) {
	if domain == nil || domain.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_cq_open")
	}

	var ca *C.struct_fi_cq_attr
	var tmp C.struct_fi_cq_attr
	format := C.enum_fi_cq_format(C.FI_CQ_FORMAT_UNSPEC)
	if attr != nil {
		tmp.size = C.size_t(attr.Size)
		tmp.flags = C.uint64_t(attr.Flags)
		tmp.format = C.enum_fi_cq_format(attr.Format)
		format = tmp.format
		tmp.wait_obj = C.enum_fi_wait_obj(attr.WaitObj)
		tmp.signaling_vector = C.int(attr.SignalingVector)
		tmp.wait_cond = C.enum_fi_cq_wait_cond(attr.WaitCondition)
		ca = &tmp
	}

	var cq *C.struct_fid_cq
	status := C.fi_cq_open(domain.ptr, ca, &cq, nil)
	if err := ErrorFromStatus(int(status), "fi_cq_open"); err != nil {
		return nil, err
	}
	return &CompletionQueue{ptr: cq, format: format}, nil
}

// Close releases the completion queue.
func (c *CompletionQueue) Close() error {
	if c == nil || c.ptr == nil {
		return nil
	}
	status := C.fi_close((*C.struct_fid)(unsafe.Pointer(c.ptr)))
	if err := ErrorFromStatus(int(status), "fi_close(cq)"); err != nil {
		return err
	}
	c.ptr = nil
	return nil
}

// ReadContext reads a single completion entry and returns its operation context.
func (c *CompletionQueue) ReadContext() (*CQEvent, error) {
	if c == nil || c.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_cq_read")
	}
	if c.format == C.FI_CQ_FORMAT_TAGGED {
		var tagged C.struct_fi_cq_tagged_entry
		ret := C.fi_cq_read(c.ptr, unsafe.Pointer(&tagged), 1)
		if ret > 0 {
			return &CQEvent{Context: tagged.op_context, Tag: uint64(tagged.tag), Data: uint64(tagged.data)}, nil
		}
		if ret == 0 {
			return nil, nil
		}
		return nil, ErrorFromStatus(int(ret), "fi_cq_read")
	}
	if c.format == C.FI_CQ_FORMAT_MSG {
		var msg C.struct_fi_cq_msg_entry
		var addr C.fi_addr_t
		ret := C.fi_cq_readfrom(c.ptr, unsafe.Pointer(&msg), 1, &addr)
		if ret > 0 {
			return &CQEvent{Context: msg.op_context, SrcAddr: uint64(addr)}, nil
		}
		if ret == 0 {
			return nil, nil
		}
		return nil, ErrorFromStatus(int(ret), "fi_cq_readfrom")
	}
	var entry C.struct_fi_cq_entry
	ret := C.fi_cq_read(c.ptr, unsafe.Pointer(&entry), 1)
	if ret > 0 {
		return &CQEvent{Context: entry.op_context}, nil
	}
	if ret == 0 {
		return nil, nil
	}
	return nil, ErrorFromStatus(int(ret), "fi_cq_read")
}

// ReadError reads a completion error entry.
func (c *CompletionQueue) ReadError(flags uint64) (*CQError, error) {
	if c == nil || c.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_cq_readerr")
	}
	var entry C.struct_fi_cq_err_entry
	ret := C.fi_cq_readerr(c.ptr, &entry, C.uint64_t(flags))
	if ret > 0 {
		err := Errno(entry.err)
		return &CQError{
			Context:     entry.op_context,
			Flags:       uint64(entry.flags),
			Length:      uint64(entry.len),
			Buffer:      entry.buf,
			Data:        uint64(entry.data),
			Tag:         uint64(entry.tag),
			Err:         err,
			ProviderErr: int(entry.prov_errno),
			ErrData:     entry.err_data,
			ErrDataSize: uint64(entry.err_data_size),
			SrcAddr:     uint64(entry.src_addr),
		}, nil
	}
	if ret == 0 {
		return nil, nil
	}
	return nil, ErrorFromStatus(int(ret), "fi_cq_readerr")
}

// OpenEventQueue opens an event queue on the provided fabric.
func OpenEventQueue(fabric *Fabric, attr *EQAttr) (*EventQueue, error) {
	if fabric == nil || fabric.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_eq_open")
	}

	var ea *C.struct_fi_eq_attr
	var tmp C.struct_fi_eq_attr
	if attr != nil {
		tmp.size = C.size_t(attr.Size)
		tmp.flags = C.uint64_t(attr.Flags)
		tmp.wait_obj = C.enum_fi_wait_obj(attr.WaitObj)
		tmp.signaling_vector = C.int(attr.SignalingVector)
		ea = &tmp
	}

	var eq *C.struct_fid_eq
	status := C.fi_eq_open(fabric.ptr, ea, &eq, nil)
	if err := ErrorFromStatus(int(status), "fi_eq_open"); err != nil {
		return nil, err
	}
	return &EventQueue{ptr: eq}, nil
}

// Close releases the event queue.
func (e *EventQueue) Close() error {
	if e == nil || e.ptr == nil {
		return nil
	}
	status := C.fi_close((*C.struct_fid)(unsafe.Pointer(e.ptr)))
	if err := ErrorFromStatus(int(status), "fi_close(eq)"); err != nil {
		return err
	}
	e.ptr = nil
	return nil
}

// Read retrieves the next event queue entry.
func (e *EventQueue) Read(flags uint64) (*EQEvent, error) {
	if e == nil || e.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_eq_read")
	}
	var code C.uint32_t
	var entry C.struct_fi_eq_entry
	ret := C.fi_eq_read(e.ptr, &code, unsafe.Pointer(&entry), C.size_t(unsafe.Sizeof(entry)), C.uint64_t(flags))
	if ret > 0 {
		return &EQEvent{
			Event:   uint32(code),
			FID:     unsafe.Pointer(entry.fid),
			Context: entry.context,
			Data:    uint64(entry.data),
		}, nil
	}
	if ret == 0 {
		return nil, nil
	}
	return nil, ErrorFromStatus(int(ret), "fi_eq_read")
}

// ReadError retrieves the next event queue error entry.
func (e *EventQueue) ReadError(flags uint64) (*EQError, error) {
	if e == nil || e.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_eq_readerr")
	}
	var entry C.struct_fi_eq_err_entry
	ret := C.fi_eq_readerr(e.ptr, &entry, C.uint64_t(flags))
	if ret > 0 {
		err := Errno(entry.err)
		return &EQError{
			FID:         unsafe.Pointer(entry.fid),
			Context:     entry.context,
			Data:        uint64(entry.data),
			Err:         err,
			ProviderErr: int(entry.prov_errno),
			ErrData:     entry.err_data,
			ErrDataSize: uint64(entry.err_data_size),
		}, nil
	}
	if ret == 0 {
		return nil, nil
	}
	return nil, ErrorFromStatus(int(ret), "fi_eq_readerr")
}

// OpenPassiveEndpoint creates a passive endpoint from the supplied descriptor info.
func OpenPassiveEndpoint(fabric *Fabric, entry InfoEntry) (*PassiveEndpoint, error) {
	if fabric == nil || fabric.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_passive_ep")
	}
	if entry.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_passive_ep")
	}
	var pep *C.struct_fid_pep
	status := C.fi_passive_ep(fabric.ptr, entry.ptr, &pep, nil)
	if err := ErrorFromStatus(int(status), "fi_passive_ep"); err != nil {
		return nil, err
	}
	return &PassiveEndpoint{ptr: pep}, nil
}

// Close releases the passive endpoint.
func (p *PassiveEndpoint) Close() error {
	if p == nil || p.ptr == nil {
		return nil
	}
	status := C.fi_close((*C.struct_fid)(unsafe.Pointer(p.ptr)))
	if err := ErrorFromStatus(int(status), "fi_close(pep)"); err != nil {
		return err
	}
	p.ptr = nil
	return nil
}

// BindEventQueue binds an event queue to the passive endpoint.
func (p *PassiveEndpoint) BindEventQueue(eq *EventQueue, flags uint64) error {
	if p == nil || p.ptr == nil || eq == nil || eq.ptr == nil {
		return ErrUnavailable.WithOp("fi_pep_bind")
	}
	status := C.fi_pep_bind(p.ptr, (*C.struct_fid)(unsafe.Pointer(eq.ptr)), C.uint64_t(flags))
	return ErrorFromStatus(int(status), "fi_pep_bind")
}

// Listen transitions the passive endpoint into a listening state.
func (p *PassiveEndpoint) Listen() error {
	if p == nil || p.ptr == nil {
		return ErrUnavailable.WithOp("fi_listen")
	}
	status := C.fi_listen(p.ptr)
	return ErrorFromStatus(int(status), "fi_listen")
}

// Name returns the provider-specific address associated with the passive endpoint.
func (p *PassiveEndpoint) Name() ([]byte, error) {
	if p == nil || p.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_getname")
	}
	size := C.size_t(128)
	for attempt := 0; attempt < 6; attempt++ {
		buf := C.malloc(size)
		if buf == nil {
			return nil, fmt.Errorf("libfabric: unable to allocate name buffer")
		}
		length := size
		status := C.fi_getname((*C.struct_fid)(unsafe.Pointer(p.ptr)), buf, &length)
		if status == 0 {
			goBytes := C.GoBytes(buf, C.int(length))
			C.free(buf)
			return goBytes, nil
		}
		C.free(buf)
		if status == -C.int(C.FI_ENOSPC) {
			size *= 2
			continue
		}
		return nil, ErrorFromStatus(int(status), "fi_getname")
	}
	return nil, fmt.Errorf("libfabric: unable to retrieve passive endpoint name")
}

// OpenEndpointWithInfo opens an active endpoint using the supplied connection info entry.
func OpenEndpointWithInfo(domain *Domain, entry InfoEntry) (*Endpoint, error) {
	if domain == nil || domain.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_endpoint")
	}
	if entry.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_endpoint")
	}
	var ep *C.struct_fid_ep
	status := C.fi_endpoint(domain.ptr, entry.ptr, &ep, nil)
	if err := ErrorFromStatus(int(status), "fi_endpoint"); err != nil {
		return nil, err
	}
	return &Endpoint{ptr: ep}, nil
}

// Accept acknowledges a pending connection request on the endpoint.
func (e *Endpoint) Accept(param unsafe.Pointer, length uintptr) error {
	if e == nil || e.ptr == nil {
		return ErrUnavailable.WithOp("fi_accept")
	}
	status := C.fi_accept(e.ptr, param, C.size_t(length))
	return ErrorFromStatus(int(status), "fi_accept")
}

// Connect initiates a connection request from the endpoint.
func (e *Endpoint) Connect(param unsafe.Pointer, length uintptr) error {
	if e == nil || e.ptr == nil {
		return ErrUnavailable.WithOp("fi_connect")
	}
	status := C.fi_connect(e.ptr, nil, param, C.size_t(length))
	return ErrorFromStatus(int(status), "fi_connect")
}

// Pointer exposes the underlying fid_ep pointer.
func (e *Endpoint) Pointer() unsafe.Pointer {
	if e == nil || e.ptr == nil {
		return nil
	}
	return unsafe.Pointer(e.ptr)
}

// FreeInfo releases a fi_info entry.
func FreeInfo(entry InfoEntry) {
	if entry.ptr == nil {
		return
	}
	C.fi_freeinfo(entry.ptr)
}

// FreeInfo releases the fi_info associated with the CM event if owned.
func (e *CMEvent) FreeInfo() {
	if e == nil || !e.ownsInfo {
		return
	}
	FreeInfo(e.Info)
	e.ownsInfo = false
}

// ReadCM retrieves the next connection management event.
func (e *EventQueue) ReadCM(timeout time.Duration) (*CMEvent, error) {
	if e == nil || e.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_eq_sread")
	}
	var code C.uint32_t
	var entry C.struct_fi_eq_cm_entry
	timeoutMs := C.int(-1)
	if timeout >= 0 {
		timeoutMs = C.int(timeout / time.Millisecond)
	}
	ret := C.fi_eq_sread(e.ptr, &code, unsafe.Pointer(&entry), C.size_t(unsafe.Sizeof(entry)), timeoutMs, 0)
	if ret > 0 {
		cm := &CMEvent{
			Event:    CMEventType(code),
			FID:      unsafe.Pointer(entry.fid),
			Info:     InfoEntry{ptr: entry.info},
			ownsInfo: entry.info != nil,
		}
		baseSize := int(C.size_t(unsafe.Sizeof(entry)))
		if dataLen := int(ret) - baseSize; dataLen > 0 {
			start := unsafe.Pointer(uintptr(unsafe.Pointer(&entry)) + uintptr(baseSize))
			cm.Data = C.GoBytes(start, C.int(dataLen))
		}
		return cm, nil
	}
	if ret == 0 {
		return nil, nil
	}
	return nil, ErrorFromStatus(int(ret), "fi_eq_sread")
}

// BindCompletionQueue binds the endpoint to a completion queue with the supplied flags.
func (e *Endpoint) BindCompletionQueue(cq *CompletionQueue, flags uint64) error {
	if e == nil || e.ptr == nil || cq == nil || cq.ptr == nil {
		return ErrUnavailable.WithOp("fi_ep_bind(cq)")
	}
	status := C.fi_ep_bind(e.ptr, (*C.struct_fid)(unsafe.Pointer(cq.ptr)), C.uint64_t(flags))
	return ErrorFromStatus(int(status), "fi_ep_bind(cq)")
}

// BindEventQueue binds the endpoint to an event queue with the supplied flags.
func (e *Endpoint) BindEventQueue(eq *EventQueue, flags uint64) error {
	if e == nil || e.ptr == nil || eq == nil || eq.ptr == nil {
		return ErrUnavailable.WithOp("fi_ep_bind(eq)")
	}
	status := C.fi_ep_bind(e.ptr, (*C.struct_fid)(unsafe.Pointer(eq.ptr)), C.uint64_t(flags))
	return ErrorFromStatus(int(status), "fi_ep_bind(eq)")
}

// Enable transitions the endpoint into an active state.
func (e *Endpoint) Enable() error {
	if e == nil || e.ptr == nil {
		return ErrUnavailable.WithOp("fi_enable")
	}
	status := C.fi_enable(e.ptr)
	return ErrorFromStatus(int(status), "fi_enable")
}

// Send posts a message send operation.
func (e *Endpoint) Send(buffer unsafe.Pointer, length uintptr, desc unsafe.Pointer, dest FIAddr, context unsafe.Pointer) error {
	if e == nil || e.ptr == nil {
		return ErrUnavailable.WithOp("fi_send")
	}
	status := C.fi_send(e.ptr, buffer, C.size_t(length), desc, C.fi_addr_t(dest), context)
	return ErrorFromStatus(int(status), "fi_send")
}

// Recv posts a message receive operation.
func (e *Endpoint) Recv(buffer unsafe.Pointer, length uintptr, desc unsafe.Pointer, src FIAddr, context unsafe.Pointer) error {
	if e == nil || e.ptr == nil {
		return ErrUnavailable.WithOp("fi_recv")
	}
	status := C.fi_recv(e.ptr, buffer, C.size_t(length), desc, C.fi_addr_t(src), context)
	return ErrorFromStatus(int(status), "fi_recv")
}

// Inject sends a message using the inject path when supported.
func (e *Endpoint) Inject(buffer unsafe.Pointer, length uintptr, dest FIAddr) error {
	if e == nil || e.ptr == nil {
		return ErrUnavailable.WithOp("fi_inject")
	}
	status := C.fi_inject(e.ptr, buffer, C.size_t(length), C.fi_addr_t(dest))
	return ErrorFromStatus(int(status), "fi_inject")
}

// InjectSizeHint returns the provider's inject size hint (currently fixed to 64 bytes).

// BindAddressVector binds the endpoint to an address vector.
func (e *Endpoint) BindAddressVector(av *AV, flags uint64) error {
	if e == nil || e.ptr == nil || av == nil || av.ptr == nil {
		return ErrUnavailable.WithOp("fi_ep_bind(av)")
	}
	status := C.fi_ep_bind(e.ptr, (*C.struct_fid)(unsafe.Pointer(av.ptr)), C.uint64_t(flags))
	return ErrorFromStatus(int(status), "fi_ep_bind(av)")
}

// Name returns the provider-specific endpoint address bytes.
func (e *Endpoint) Name() ([]byte, error) {
	if e == nil || e.ptr == nil {
		return nil, ErrUnavailable.WithOp("fi_getname")
	}
	size := C.size_t(128)
	for attempt := 0; attempt < 6; attempt++ {
		buf := C.malloc(size)
		if buf == nil {
			return nil, fmt.Errorf("libfabric: unable to allocate name buffer")
		}
		length := size
		status := C.fi_getname((*C.struct_fid)(unsafe.Pointer(e.ptr)), buf, &length)
		if status == 0 {
			goBytes := C.GoBytes(buf, C.int(length))
			C.free(buf)
			return goBytes, nil
		}
		C.free(buf)
		if status == -C.int(C.FI_ENOSPC) {
			size *= 2
			continue
		}
		return nil, ErrorFromStatus(int(status), "fi_getname")
	}
	return nil, fmt.Errorf("libfabric: unable to retrieve endpoint name")
}
