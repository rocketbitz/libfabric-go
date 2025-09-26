package fi

import (
	"fmt"

	"github.com/rocketbitz/libfabric-go/internal/capi"
)

// EndpointType re-exports capi.EndpointType for consumers of the public API.
type EndpointType = capi.EndpointType

const (
	EndpointTypeUnspec = capi.EndpointTypeUnspec
	EndpointTypeMsg    = capi.EndpointTypeMsg
	EndpointTypeDgram  = capi.EndpointTypeDgram
	EndpointTypeRDM    = capi.EndpointTypeRDM
)

const (
	CapMsg         = capi.CapMsg
	CapTagged      = capi.CapTagged
	CapRMA         = capi.CapRMA
	CapAtomic      = capi.CapAtomic
	CapInject      = capi.CapInject
	CapRemoteRead  = capi.CapRemoteRead
	CapRemoteWrite = capi.CapRemoteWrite
)

// MRModeFlag represents provider memory-registration requirements.
type MRModeFlag uint64

const (
	MRModeLocal      MRModeFlag = MRModeFlag(capi.MRModeLocal)
	MRModeRaw        MRModeFlag = MRModeFlag(capi.MRModeRaw)
	MRModeVirtAddr   MRModeFlag = MRModeFlag(capi.MRModeVirtAddr)
	MRModeAllocated  MRModeFlag = MRModeFlag(capi.MRModeAllocated)
	MRModeProvKey    MRModeFlag = MRModeFlag(capi.MRModeProvKey)
	MRModeMMUNotify  MRModeFlag = MRModeFlag(capi.MRModeMMUNotify)
	MRModeRMAEvent   MRModeFlag = MRModeFlag(capi.MRModeRMAEvent)
	MRModeEndpoint   MRModeFlag = MRModeFlag(capi.MRModeEndpoint)
	MRModeHMEM       MRModeFlag = MRModeFlag(capi.MRModeHMEM)
	MRModeCollective MRModeFlag = MRModeFlag(capi.MRModeCollective)
)

// Info captures a Go-friendly snapshot of an fi_info descriptor produced during
// provider discovery.
type Info struct {
	Provider        string
	Fabric          string
	Domain          string
	Caps            uint64
	Mode            uint64
	Endpoint        EndpointType
	ProviderVersion capi.Version
	APIVersion      capi.Version
	InjectSize      uintptr
	MRMode          uint64
	MRKeySize       uintptr
	MRIovLimit      uintptr
}

// SupportsCap reports whether the specified capability bit is set.
func (i Info) SupportsCap(flag uint64) bool {
	return i.Caps&flag != 0
}

// SupportsTagged indicates whether the provider advertises tagged messaging support.
func (i Info) SupportsTagged() bool {
	return i.SupportsCap(capi.CapTagged)
}

// SupportsMsg indicates whether standard message operations are available.
func (i Info) SupportsMsg() bool {
	return i.SupportsCap(capi.CapMsg)
}

// SupportsRMA reports whether the provider advertises remote memory access support.
func (i Info) SupportsRMA() bool {
	return i.SupportsCap(capi.CapRMA)
}

// MRModeFlags returns the raw provider MR mode bits.
func (i Info) MRModeFlags() MRModeFlag {
	return MRModeFlag(i.MRMode)
}

// RequiresMRMode reports whether the provider requires the specified MR mode flag.
func (i Info) RequiresMRMode(flag MRModeFlag) bool {
	if flag == 0 {
		return false
	}
	return i.MRMode&uint64(flag) != 0
}

// SupportsRemoteRead reports whether remote read operations are available.
func (i Info) SupportsRemoteRead() bool {
	return i.SupportsCap(capi.CapRemoteRead)
}

// SupportsRemoteWrite reports whether remote write operations are available.
func (i Info) SupportsRemoteWrite() bool {
	return i.SupportsCap(capi.CapRemoteWrite)
}

// SupportsEndpointType reports whether this entry targets the specified endpoint type.
func (i Info) SupportsEndpointType(ep EndpointType) bool {
	return i.Endpoint == ep
}

// SupportsRDM indicates whether the entry describes a reliable datagram endpoint.
func (i Info) SupportsRDM() bool {
	return i.SupportsEndpointType(EndpointTypeRDM)
}

// SupportsDatagram indicates whether the entry describes a datagram endpoint.
func (i Info) SupportsDatagram() bool {
	return i.SupportsEndpointType(EndpointTypeDgram)
}

// DiscoverOption adjusts discovery behavior.
type DiscoverOption func(*discoverConfig)

type discoverConfig struct {
	version      capi.Version
	node         string
	service      string
	flags        uint64
	provider     string
	fabric       string
	domain       string
	endpointType *EndpointType
	caps         *uint64
	mode         *uint64
}

func defaultDiscoverConfig() discoverConfig {
	return discoverConfig{
		version: capi.BuildVersion(),
	}
}

func (c *discoverConfig) needsHints() bool {
	return c.provider != "" || c.fabric != "" || c.domain != "" || c.endpointType != nil || c.caps != nil || c.mode != nil
}

func (c *discoverConfig) applyHints(info *capi.Info) {
	if !c.needsHints() {
		return
	}
	if c.provider != "" {
		info.SetProvider(c.provider)
	}
	if c.fabric != "" {
		info.SetFabricName(c.fabric)
	}
	if c.domain != "" {
		info.SetDomainName(c.domain)
	}
	if c.endpointType != nil {
		info.SetEndpointType(*c.endpointType)
	}
	if c.caps != nil {
		info.SetCaps(*c.caps)
	}
	if c.mode != nil {
		info.SetMode(*c.mode)
	}
}

// WithNode specifies the node parameter for discovery.
func WithNode(node string) DiscoverOption {
	return func(cfg *discoverConfig) {
		cfg.node = node
	}
}

// WithService specifies the service (port) parameter for discovery.
func WithService(service string) DiscoverOption {
	return func(cfg *discoverConfig) {
		cfg.service = service
	}
}

// WithFlags sets the flags passed into fi_getinfo.
func WithFlags(flags uint64) DiscoverOption {
	return func(cfg *discoverConfig) {
		cfg.flags = flags
	}
}

// WithProvider filters discovery by provider name.
func WithProvider(provider string) DiscoverOption {
	return func(cfg *discoverConfig) {
		cfg.provider = provider
	}
}

// WithFabric filters discovery by fabric name.
func WithFabric(name string) DiscoverOption {
	return func(cfg *discoverConfig) {
		cfg.fabric = name
	}
}

// WithDomain filters discovery by domain name.
func WithDomain(name string) DiscoverOption {
	return func(cfg *discoverConfig) {
		cfg.domain = name
	}
}

// WithEndpointType requests descriptors compatible with the specified endpoint type.
func WithEndpointType(ep EndpointType) DiscoverOption {
	return func(cfg *discoverConfig) {
		cfg.endpointType = new(EndpointType)
		*cfg.endpointType = ep
	}
}

// WithCaps sets the required capabilities bitmask.
func WithCaps(caps uint64) DiscoverOption {
	return func(cfg *discoverConfig) {
		cfg.caps = new(uint64)
		*cfg.caps = caps
	}
}

// WithMode sets the required mode bitmask.
func WithMode(mode uint64) DiscoverOption {
	return func(cfg *discoverConfig) {
		cfg.mode = new(uint64)
		*cfg.mode = mode
	}
}

// WithVersion overrides the version used when querying providers.
func WithVersion(ver capi.Version) DiscoverOption {
	return func(cfg *discoverConfig) {
		cfg.version = ver
	}
}

func infoFromEntry(entry capi.InfoEntry) Info {
	return Info{
		Provider:        entry.ProviderName(),
		Fabric:          entry.FabricName(),
		Domain:          entry.DomainName(),
		Caps:            entry.Caps(),
		Mode:            entry.Mode(),
		Endpoint:        EndpointType(entry.EndpointType()),
		ProviderVersion: entry.ProviderVersion(),
		APIVersion:      entry.APIVersion(),
		InjectSize:      entry.InjectSize(),
		MRMode:          entry.MRMode(),
		MRKeySize:       entry.MRKeySize(),
		MRIovLimit:      entry.MRIovLimit(),
	}
}

// Discovery retains ownership of the underlying fi_info list so that
// descriptors can be used to open additional resources. Call Close when done.
type Discovery struct {
	info *capi.Info
}

// Close releases the underlying fi_info resources.
func (d *Discovery) Close() {
	if d == nil || d.info == nil {
		return
	}
	d.info.Free()
	d.info = nil
}

// Descriptor snapshots a single fi_info entry. It is valid as long as the
// parent Discovery remains open.
type Descriptor struct {
	entry capi.InfoEntry
}

// Info returns a value snapshot for the descriptor.
func (d Descriptor) Info() Info {
	return infoFromEntry(d.entry)
}

// Provider exposes the provider name directly.
func (d Descriptor) Provider() string {
	return d.entry.ProviderName()
}

// SupportsTagged reports whether the descriptor's provider supports tagged messaging.
func (d Descriptor) SupportsTagged() bool {
	return d.entry.Caps()&capi.CapTagged != 0
}

// SupportsMsg reports whether standard messaging is supported.
func (d Descriptor) SupportsMsg() bool {
	return d.entry.Caps()&capi.CapMsg != 0
}

// SupportsRMA reports whether the descriptor advertises RMA support.
func (d Descriptor) SupportsRMA() bool {
	return d.entry.Caps()&capi.CapRMA != 0
}

// SupportsRemoteRead reports whether remote read operations are available.
func (d Descriptor) SupportsRemoteRead() bool {
	return d.entry.Caps()&capi.CapRemoteRead != 0
}

// SupportsRemoteWrite reports whether remote write operations are available.
func (d Descriptor) SupportsRemoteWrite() bool {
	return d.entry.Caps()&capi.CapRemoteWrite != 0
}

// MRModeFlags returns the raw provider MR mode bits.
func (d Descriptor) MRModeFlags() MRModeFlag {
	return MRModeFlag(d.entry.MRMode())
}

// RequiresMRMode reports whether the descriptor requires the specified MR mode flag.
func (d Descriptor) RequiresMRMode(flag MRModeFlag) bool {
	if flag == 0 {
		return false
	}
	return d.entry.MRMode()&uint64(flag) != 0
}

// MRKeySize returns the provider-specified memory registration key size.
func (d Descriptor) MRKeySize() uintptr {
	return d.entry.MRKeySize()
}

// MRIovLimit returns the provider's limit for iov-based registrations.
func (d Descriptor) MRIovLimit() uintptr {
	return d.entry.MRIovLimit()
}

// EndpointType returns the endpoint type associated with this descriptor.
func (d Descriptor) EndpointType() EndpointType {
	return EndpointType(d.entry.EndpointType())
}

// SupportsEndpointType reports whether the descriptor targets the specified endpoint type.
func (d Descriptor) SupportsEndpointType(t EndpointType) bool {
	return d.EndpointType() == t
}

// Descriptors returns all entries within the discovery result.
func (d *Discovery) Descriptors() []Descriptor {
	if d == nil || d.info == nil {
		return nil
	}
	entries := d.info.Entries()
	res := make([]Descriptor, len(entries))
	for i, entry := range entries {
		res[i] = Descriptor{entry: entry}
	}
	return res
}

// SupportsEndpointType reports whether any descriptor within the discovery result supports the specified endpoint type.
func (d *Discovery) SupportsEndpointType(t EndpointType) bool {
	for _, desc := range d.Descriptors() {
		if desc.SupportsEndpointType(t) {
			return true
		}
	}
	return false
}

// DiscoverDescriptors performs discovery and returns a handle that can open
// fabrics or domains. Call Close on the returned handle to release resources.
func DiscoverDescriptors(opts ...DiscoverOption) (*Discovery, error) {
	if err := capi.EnsureRuntimeCompatible(); err != nil {
		return nil, err
	}
	cfg := defaultDiscoverConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	var hints *capi.Info
	if cfg.needsHints() {
		hints = capi.AllocInfo()
		cfg.applyHints(hints)
		defer hints.Free()
	}

	list, err := capi.GetInfo(cfg.version, cfg.node, cfg.service, cfg.flags, hints)
	if err != nil {
		return nil, err
	}
	return &Discovery{info: list}, nil
}

// Discover queries libfabric for provider descriptors using fi_getinfo and
// returns value snapshots. For resource operations use DiscoverDescriptors.
func Discover(opts ...DiscoverOption) ([]Info, error) {
	result, err := DiscoverDescriptors(opts...)
	if err != nil {
		return nil, err
	}
	defer result.Close()

	descriptors := result.Descriptors()
	infos := make([]Info, len(descriptors))
	for i, descriptor := range descriptors {
		infos[i] = descriptor.Info()
	}

	return infos, nil
}

// Fabric wraps a libfabric fid_fabric handle.
type Fabric struct {
	handle *capi.Fabric
}

// Close releases the underlying fabric handle.
func (f *Fabric) Close() error {
	if f == nil || f.handle == nil {
		return nil
	}
	err := f.handle.Close()
	f.handle = nil
	return err
}

// Domain wraps a libfabric fid_domain handle.
type Domain struct {
	handle     *capi.Domain
	mrMode     uint64
	mrKeySize  uintptr
	mrIovLimit uintptr
}

// MRModeFlags reports the domain's memory registration mode requirements.
func (d *Domain) MRModeFlags() MRModeFlag {
	if d == nil {
		return 0
	}
	return MRModeFlag(d.mrMode)
}

// RequiresMRMode reports whether the domain requires the specified MR mode flag.
func (d *Domain) RequiresMRMode(flag MRModeFlag) bool {
	if d == nil || flag == 0 {
		return false
	}
	return d.mrMode&uint64(flag) != 0
}

// MRKeySize reports the provider-specified memory registration key size, if any.
func (d *Domain) MRKeySize() uintptr {
	if d == nil {
		return 0
	}
	return d.mrKeySize
}

// MRIovLimit reports the provider's iov registration limit when advertised.
func (d *Domain) MRIovLimit() uintptr {
	if d == nil {
		return 0
	}
	return d.mrIovLimit
}

// Close releases the underlying domain handle.
func (d *Domain) Close() error {
	if d == nil || d.handle == nil {
		return nil
	}
	err := d.handle.Close()
	d.handle = nil
	return err
}

// OpenFabric opens a fabric for the descriptor.
func (d Descriptor) OpenFabric() (*Fabric, error) {
	fabric, err := capi.OpenFabric(d.entry)
	if err != nil {
		return nil, err
	}
	return &Fabric{handle: fabric}, nil
}

// OpenDomain opens a domain associated with the provided fabric and descriptor.
func (d Descriptor) OpenDomain(fabric *Fabric) (*Domain, error) {
	if fabric == nil || fabric.handle == nil {
		return nil, ErrInvalidHandle{"fabric"}
	}
	dom, err := capi.OpenDomain(fabric.handle, d.entry)
	if err != nil {
		return nil, err
	}
	return &Domain{
		handle:     dom,
		mrMode:     d.entry.MRMode(),
		mrKeySize:  d.entry.MRKeySize(),
		mrIovLimit: d.entry.MRIovLimit(),
	}, nil
}

// EnsureRuntimeAtLeast wraps the capi variant for convenience.
func EnsureRuntimeAtLeast(ver capi.Version) error {
	return capi.EnsureRuntimeAtLeast(ver)
}

// RuntimeVersion proxies to the capi layer.
func RuntimeVersion() capi.Version {
	return capi.RuntimeVersion()
}

// FormatInfo provides a readable representation of the descriptor information.
func FormatInfo(info Info) string {
	return fmt.Sprintf("provider=%s fabric=%s domain=%s endpoint=%s", info.Provider, info.Fabric, info.Domain, info.Endpoint)
}
