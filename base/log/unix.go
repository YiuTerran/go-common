//go:build !windows

package log

import (
	"os"

	"golang.org/x/sys/unix"
)

// dupFD is used to initialize OrigStderr (see stderr_redirect.go).
func dupFD(fd uintptr) (uintptr, error) {
	// Warning: failing to set FD_CLOEXEC causes the duplicated file descriptor
	// to leak into subprocesses created by exec.Command. If the file descriptor
	// is a pipe, these subprocesses will hold the pipe open (i.e., prevent
	// EOF), potentially beyond the lifetime of this process.
	//
	// This can break go test's timeouts. go test usually spawns a test process
	// with its stdin and stderr streams hooked up to pipes; if the test process
	// times out, it sends a SIGKILL and attempts to read stdin and stderr to
	// completion. If the test process has itself spawned long-lived
	// subprocesses that hold references to the stdin or stderr pipes, go test
	// will hang until the subprocesses exit, rather defeating the purpose of
	// a timeout.
	nfd, err := unix.FcntlInt(fd, unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		return 0, err
	}
	return uintptr(nfd), nil
}

// redirectStderr is used to redirect internal writes to fd 2 to the
// specified file. This is needed to ensure that hardcoded writes to fd
// 2 by e.g. the Go runtime are redirected to a log file of our
// choosing.
//
// We also override os.Stderr for those other parts of Go which use
// that and not fd 2 directly.
func redirectStderr(errorFile string) error {
	f, err := os.OpenFile(errorFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	if err := unix.Dup2(int(f.Fd()), unix.Stderr); err != nil {
		return err
	}
	os.Stderr = f
	return nil
}
