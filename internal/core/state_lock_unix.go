//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package core

import (
	"os"
	"syscall"
)

func lockStateFile(f *os.File) (func(), error) {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return nil, err
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }, nil
}
