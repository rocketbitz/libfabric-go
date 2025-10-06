//go:build cgo

package capi

import "fmt"

/*
#cgo pkg-config: libfabric
#include <rdma/fabric.h>

static inline unsigned int libfabric_version(void) {
    return fi_version();
}

static inline unsigned int libfabric_version_major(unsigned int v) {
    return FI_MAJOR(v);
}

static inline unsigned int libfabric_version_minor(unsigned int v) {
    return FI_MINOR(v);
}

static inline unsigned int libfabric_build_version_major(void) {
    return FI_MAJOR_VERSION;
}

static inline unsigned int libfabric_build_version_minor(void) {
    return FI_MINOR_VERSION;
}
*/
import "C"

// Version represents a libfabric semantic version.
type Version struct {
	Major uint
	Minor uint
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// Compare returns -1 if v < other, 0 if equal, and 1 if v > other.
func (v Version) Compare(other Version) int {
	switch {
	case v.Major < other.Major:
		return -1
	case v.Major > other.Major:
		return 1
	case v.Minor < other.Minor:
		return -1
	case v.Minor > other.Minor:
		return 1
	default:
		return 0
	}
}

// RuntimeVersion queries the libfabric version reported by the linked runtime.
func RuntimeVersion() Version {
	ver := C.libfabric_version()
	return Version{
		Major: uint(C.libfabric_version_major(ver)),
		Minor: uint(C.libfabric_version_minor(ver)),
	}
}

// BuildVersion reports the libfabric version encoded in the headers used at
// compile time. This can diverge from RuntimeVersion when linked against a
// different library release at runtime.
func BuildVersion() Version {
	return Version{
		Major: uint(C.libfabric_build_version_major()),
		Minor: uint(C.libfabric_build_version_minor()),
	}
}

// EnsureRuntimeAtLeast validates that the runtime libfabric is at least the
// provided version.
func EnsureRuntimeAtLeast(minimum Version) error {
	runtime := RuntimeVersion()
	if runtime.Compare(minimum) < 0 {
		return fmt.Errorf("libfabric runtime %s is older than required %s", runtime, minimum)
	}
	return nil
}

// EnsureRuntimeCompatible ensures the runtime version matches or exceeds the
// headers' minor version within the same major release.
func EnsureRuntimeCompatible() error {
	build := BuildVersion()
	runtime := RuntimeVersion()

	if runtime.Major != build.Major {
		return fmt.Errorf("libfabric major version mismatch: runtime %s, headers %s", runtime, build)
	}
	if runtime.Minor < build.Minor {
		return fmt.Errorf("libfabric runtime %s predates header minor version %s", runtime, build)
	}
	return nil
}
