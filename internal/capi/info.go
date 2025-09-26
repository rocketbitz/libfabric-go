//go:build cgo

package capi

import (
	"unsafe"
)

/*
#cgo pkg-config: libfabric
#include <stdlib.h>
#include <rdma/fabric.h>

static inline uint32_t go_fi_version(unsigned int major, unsigned int minor) {
    return FI_VERSION(major, minor);
}

static inline unsigned int go_fi_major(uint32_t version) {
    return FI_MAJOR(version);
}

static inline unsigned int go_fi_minor(uint32_t version) {
    return FI_MINOR(version);
}

static inline struct fi_fabric_attr* go_alloc_fabric_attr(void) {
    return calloc(1, sizeof(struct fi_fabric_attr));
}

static inline struct fi_domain_attr* go_alloc_domain_attr(void) {
    return calloc(1, sizeof(struct fi_domain_attr));
}

static inline struct fi_ep_attr* go_alloc_ep_attr(void) {
    return calloc(1, sizeof(struct fi_ep_attr));
}
*/
import "C"

// Info represents an fi_info descriptor list returned by fi_getinfo. The head
// of the list owns the underlying C allocation and must be freed with Free.
type Info struct {
	ptr  *C.struct_fi_info
	owns bool
}

func (i *Info) ensureFabricAttr() *C.struct_fi_fabric_attr {
	if i == nil || i.ptr == nil {
		return nil
	}
	if i.ptr.fabric_attr == nil {
		i.ptr.fabric_attr = C.go_alloc_fabric_attr()
	}
	return i.ptr.fabric_attr
}

func (i *Info) ensureDomainAttr() *C.struct_fi_domain_attr {
	if i == nil || i.ptr == nil {
		return nil
	}
	if i.ptr.domain_attr == nil {
		i.ptr.domain_attr = C.go_alloc_domain_attr()
	}
	return i.ptr.domain_attr
}

func (i *Info) ensureEPAttr() *C.struct_fi_ep_attr {
	if i == nil || i.ptr == nil {
		return nil
	}
	if i.ptr.ep_attr == nil {
		i.ptr.ep_attr = C.go_alloc_ep_attr()
	}
	return i.ptr.ep_attr
}

// EndpointType mirrors enum fi_ep_type from libfabric headers.
type EndpointType int

const (
	EndpointTypeUnspec EndpointType = EndpointType(C.FI_EP_UNSPEC)
	EndpointTypeMsg    EndpointType = EndpointType(C.FI_EP_MSG)
	EndpointTypeDgram  EndpointType = EndpointType(C.FI_EP_DGRAM)
	EndpointTypeRDM    EndpointType = EndpointType(C.FI_EP_RDM)
)

func (e EndpointType) String() string {
	switch e {
	case EndpointTypeUnspec:
		return "unspec"
	case EndpointTypeMsg:
		return "msg"
	case EndpointTypeDgram:
		return "dgram"
	case EndpointTypeRDM:
		return "rdm"
	default:
		return "unknown"
	}
}

// GetInfo wraps fi_getinfo and returns the resulting descriptor list. Callers
// must free the returned Info via Free to release native resources.
func GetInfo(ver Version, node, service string, flags uint64, hints *Info) (*Info, error) {
	var cNode, cService *C.char
	if node != "" {
		cNode = C.CString(node)
		defer C.free(unsafe.Pointer(cNode))
	}
	if service != "" {
		cService = C.CString(service)
		defer C.free(unsafe.Pointer(cService))
	}

	var hintPtr *C.struct_fi_info
	if hints != nil {
		hintPtr = hints.ptr
	}

	var out *C.struct_fi_info
	status := C.fi_getinfo(
		C.uint(C.go_fi_version(C.uint(ver.Major), C.uint(ver.Minor))),
		cNode,
		cService,
		C.uint64_t(flags),
		hintPtr,
		&out,
	)
	if err := ErrorFromStatus(int(status), "fi_getinfo"); err != nil {
		return nil, err
	}
	return &Info{ptr: out, owns: true}, nil
}

// AllocInfo allocates an empty fi_info structure suitable for use as hints.
func AllocInfo() *Info {
	return &Info{ptr: C.fi_allocinfo(), owns: true}
}

// SetProvider restricts discovery to the specified provider name.
func (i *Info) SetProvider(provider string) {
	attr := i.ensureFabricAttr()
	if attr == nil {
		return
	}
	if attr.prov_name != nil {
		C.free(unsafe.Pointer(attr.prov_name))
		attr.prov_name = nil
	}
	if provider == "" {
		return
	}
	attr.prov_name = C.CString(provider)
}

// SetDomainName sets the domain name hint.
func (i *Info) SetDomainName(name string) {
	attr := i.ensureDomainAttr()
	if attr == nil {
		return
	}
	if attr.name != nil {
		C.free(unsafe.Pointer(attr.name))
		attr.name = nil
	}
	if name == "" {
		return
	}
	attr.name = C.CString(name)
}

// SetFabricName sets the fabric name hint.
func (i *Info) SetFabricName(name string) {
	attr := i.ensureFabricAttr()
	if attr == nil {
		return
	}
	if attr.name != nil {
		C.free(unsafe.Pointer(attr.name))
		attr.name = nil
	}
	if name == "" {
		return
	}
	attr.name = C.CString(name)
}

// SetCaps assigns the requested capabilities mask.
func (i *Info) SetCaps(caps uint64) {
	if i == nil || i.ptr == nil {
		return
	}
	i.ptr.caps = C.uint64_t(caps)
}

// SetMode assigns the requested mode mask.
func (i *Info) SetMode(mode uint64) {
	if i == nil || i.ptr == nil {
		return
	}
	i.ptr.mode = C.uint64_t(mode)
}

// SetEndpointType sets the endpoint type hint.
func (i *Info) SetEndpointType(ep EndpointType) {
	attr := i.ensureEPAttr()
	if attr == nil {
		return
	}
	attr._type = C.enum_fi_ep_type(ep)
}

// Free releases the fi_info list if this Info owns the pointer.
func (i *Info) Free() {
	if i == nil || i.ptr == nil || !i.owns {
		return
	}
	C.fi_freeinfo(i.ptr)
	i.ptr = nil
	i.owns = false
}

// Entries returns a snapshot of the descriptor list for inspection.
func (i *Info) Entries() []InfoEntry {
	if i == nil || i.ptr == nil {
		return nil
	}
	var entries []InfoEntry
	for cur := i.ptr; cur != nil; cur = cur.next {
		entries = append(entries, InfoEntry{ptr: cur})
	}
	return entries
}

// InfoEntry provides read-only accessors for a single fi_info node.
type InfoEntry struct {
	ptr *C.struct_fi_info
}

// ProviderName returns the provider string, if available.
func (e InfoEntry) ProviderName() string {
	if e.ptr == nil || e.ptr.fabric_attr == nil || e.ptr.fabric_attr.prov_name == nil {
		return ""
	}
	return C.GoString(e.ptr.fabric_attr.prov_name)
}

// FabricName returns the fabric name, if available.
func (e InfoEntry) FabricName() string {
	if e.ptr == nil || e.ptr.fabric_attr == nil || e.ptr.fabric_attr.name == nil {
		return ""
	}
	return C.GoString(e.ptr.fabric_attr.name)
}

// DomainName returns the domain name, if available.
func (e InfoEntry) DomainName() string {
	if e.ptr == nil || e.ptr.domain_attr == nil || e.ptr.domain_attr.name == nil {
		return ""
	}
	return C.GoString(e.ptr.domain_attr.name)
}

// Caps returns the capabilities bitmask.
func (e InfoEntry) Caps() uint64 {
	if e.ptr == nil {
		return 0
	}
	return uint64(e.ptr.caps)
}

// Mode returns the mode bitmask.
func (e InfoEntry) Mode() uint64 {
	if e.ptr == nil {
		return 0
	}
	return uint64(e.ptr.mode)
}

// EndpointType reports the endpoint type requested by the descriptor.
func (e InfoEntry) EndpointType() EndpointType {
	if e.ptr == nil || e.ptr.ep_attr == nil {
		return EndpointTypeUnspec
	}
	return EndpointType(e.ptr.ep_attr._type)
}

// ProviderVersion reports the provider version encoded within the descriptor.
func (e InfoEntry) ProviderVersion() Version {
	if e.ptr == nil || e.ptr.fabric_attr == nil {
		return Version{}
	}
	ver := e.ptr.fabric_attr.prov_version
	return Version{
		Major: uint(C.go_fi_major(ver)),
		Minor: uint(C.go_fi_minor(ver)),
	}
}

// APIVersion reports the interface version the provider requested.
func (e InfoEntry) APIVersion() Version {
	if e.ptr == nil || e.ptr.fabric_attr == nil {
		return Version{}
	}
	ver := e.ptr.fabric_attr.api_version
	return Version{
		Major: uint(C.go_fi_major(ver)),
		Minor: uint(C.go_fi_minor(ver)),
	}
}

// InjectSize returns the provider's reported inject size hint, if available.
func (e InfoEntry) InjectSize() uintptr {
	if e.ptr == nil || e.ptr.tx_attr == nil {
		return 0
	}
	return uintptr(e.ptr.tx_attr.inject_size)
}

func (e InfoEntry) domainAttr() *C.struct_fi_domain_attr {
	if e.ptr == nil {
		return nil
	}
	return e.ptr.domain_attr
}

// MRMode reports the domain's required memory registration mode bits.
func (e InfoEntry) MRMode() uint64 {
	attr := e.domainAttr()
	if attr == nil {
		return 0
	}
	return uint64(attr.mr_mode)
}

// MRKeySize reports the memory registration key size if one is required.
func (e InfoEntry) MRKeySize() uintptr {
	attr := e.domainAttr()
	if attr == nil {
		return 0
	}
	return uintptr(attr.mr_key_size)
}

// MRIovLimit reports the provider advertised iov limit for memory registration.
func (e InfoEntry) MRIovLimit() uintptr {
	attr := e.domainAttr()
	if attr == nil {
		return 0
	}
	return uintptr(attr.mr_iov_limit)
}

// RawPointer exposes the underlying C pointer for advanced use.
func (e InfoEntry) RawPointer() *C.struct_fi_info {
	return e.ptr
}
