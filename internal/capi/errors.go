//go:build cgo

package capi

import "fmt"

/*
#cgo pkg-config: libfabric
#include <rdma/fi_errno.h>

#ifndef FI_EPROTO
#define FI_EPROTO FI_EOTHER
#endif

#ifndef FI_ENOMR
#define FI_ENOMR FI_EOTHER
#endif
*/
import "C"

// Errno represents a libfabric error code (positive integral value).
type Errno int32

// Error codes mirrored from <rdma/fi_errno.h>. This list covers common return values
// we expect to surface through the Go bindings. Additional codes can be added as
// new APIs are wrapped.
const (
	Success         Errno = Errno(C.FI_SUCCESS)
	ErrAgain        Errno = Errno(C.FI_EAGAIN)
	ErrWouldBlock   Errno = Errno(C.FI_EWOULDBLOCK)
	ErrNoMemory     Errno = Errno(C.FI_ENOMEM)
	ErrNoDevice     Errno = Errno(C.FI_ENODEV)
	ErrNoData       Errno = Errno(C.FI_ENODATA)
	ErrOpNotSupp    Errno = Errno(C.FI_EOPNOTSUPP)
	ErrNotSupported Errno = Errno(C.FI_ENOSYS)
	ErrInProgress   Errno = Errno(C.FI_EINPROGRESS)
	ErrAlready      Errno = Errno(C.FI_EALREADY)
	ErrAddrInUse    Errno = Errno(C.FI_EADDRINUSE)
	ErrAddrNotAvail Errno = Errno(C.FI_EADDRNOTAVAIL)
	ErrTimedOut     Errno = Errno(C.FI_ETIMEDOUT)
	ErrConnReset    Errno = Errno(C.FI_ECONNRESET)
	ErrConnRefused  Errno = Errno(C.FI_ECONNREFUSED)
	ErrProto        Errno = Errno(C.FI_EPROTO)
	ErrOverflow     Errno = Errno(C.FI_EOVERFLOW)
	ErrMsgSize      Errno = Errno(C.FI_EMSGSIZE)
	ErrNoProtoOpt   Errno = Errno(C.FI_ENOPROTOOPT)
	ErrOther        Errno = Errno(C.FI_EOTHER)
	ErrTooSmall     Errno = Errno(C.FI_ETOOSMALL)
	ErrBadState     Errno = Errno(C.FI_EOPBADSTATE)
	ErrUnavailable  Errno = Errno(C.FI_EAVAIL)
	ErrBadFlags     Errno = Errno(C.FI_EBADFLAGS)
	ErrNoEQ         Errno = Errno(C.FI_ENOEQ)
	ErrDomain       Errno = Errno(C.FI_EDOMAIN)
	ErrNoCQ         Errno = Errno(C.FI_ENOCQ)
	ErrCRC          Errno = Errno(C.FI_ECRC)
	ErrTrunc        Errno = Errno(C.FI_ETRUNC)
	ErrNoKey        Errno = Errno(C.FI_ENOKEY)
	ErrNoAV         Errno = Errno(C.FI_ENOAV)
	ErrOverrun      Errno = Errno(C.FI_EOVERRUN)
	ErrNoRX         Errno = Errno(C.FI_ENORX)
	ErrNoMR         Errno = Errno(C.FI_ENOMR)
)

// Error returns the human-readable string as produced by fi_strerror.
func (e Errno) Error() string {
	return e.String()
}

// String returns the libfabric-provided message for the Errno.
func (e Errno) String() string {
	if e == Success {
		return "success"
	}
	return C.GoString(C.fi_strerror(C.int(e)))
}

// WithOp adds operation context to the provided Errno.
func (e Errno) WithOp(op string) error {
	if op == "" {
		return e
	}
	return fmt.Errorf("%s: %w", op, e)
}

// ErrorFromStatus converts a libfabric status code into a Go error. Status values
// from libfabric APIs are expected to be 0 on success, negative on failure. Positive
// values are returned as-is and treated as success, e.g. when an API returns a byte
// count. FI_EAVAIL is surfaced as ErrUnavailable to signal consumers to inspect
// completion/event queues for detailed error info.
func ErrorFromStatus(status int, op string) error {
	if status >= 0 {
		return nil
	}

	code := Errno(-status)
	if code == Success {
		return nil
	}
	return code.WithOp(op)
}

// MustSucceed panics if the status represents an error. Intended for tests or
// bootstrapping code paths where failure is fatal.
func MustSucceed(status int, op string) {
	if err := ErrorFromStatus(status, op); err != nil {
		panic(err)
	}
}
