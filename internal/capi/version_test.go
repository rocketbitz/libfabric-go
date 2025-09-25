//go:build cgo

package capi

import "testing"

func TestRuntimeVersionNonZero(t *testing.T) {
	runtime := RuntimeVersion()
	if runtime.Major == 0 {
		t.Fatalf("unexpected runtime major version: %+v", runtime)
	}
	if runtime.String() == "0.0" {
		t.Fatalf("unexpected runtime version string: %s", runtime.String())
	}
}

func TestEnsureRuntimeCompatible(t *testing.T) {
	if err := EnsureRuntimeCompatible(); err != nil {
		t.Fatalf("EnsureRuntimeCompatible failed: %v", err)
	}

	build := BuildVersion()
	if err := EnsureRuntimeAtLeast(build); err != nil {
		t.Fatalf("EnsureRuntimeAtLeast failed for build version %s: %v", build, err)
	}
}
