package fi

import (
	"errors"
	"testing"

	"github.com/rocketbitz/libfabric-go/internal/capi"
)

func TestWaitSetOpenSockets(t *testing.T) {
	_, fabric, _ := setupSocketsResources(t)

	ws, err := fabric.OpenWaitSet(&WaitAttr{WaitObj: WaitFD})
	if err != nil {
		if errors.Is(err, capi.ErrNotSupported) {
			t.Skip("wait sets not supported by provider")
		}
		t.Fatalf("OpenWaitSet failed: %v", err)
	}
	t.Cleanup(func() {
		_ = ws.Close()
	})

	fd, err := ws.FD()
	if err != nil {
		if errors.Is(err, capi.ErrNotSupported) {
			t.Skip("wait set FD not supported")
		}
		t.Fatalf("WaitSet.FD failed: %v", err)
	}
	if fd < 0 {
		t.Fatalf("expected non-negative FD, got %d", fd)
	}
}
