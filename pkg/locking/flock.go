package locking

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/golang/glog"
)

type FSFlockLock struct {
	p string
}

var _ Lock = &FSFlockLock{}

type FSFlockLockGuard struct {
	p    string
	file *os.File
}

var _ LockGuard = &FSFlockLockGuard{}

func NewFSFlockLock(p string) (*FSFlockLock, error) {
	return &FSFlockLock{
		p: p,
	}, nil
}

func (l *FSFlockLock) Acquire(ctx context.Context, id string) (LockGuard, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	//data := &LockInfo{
	//	Holder: id,
	//	Timestamp:   time.Now().Unix(),
	//}
	//b, err := data.ToJSON()
	//if err != nil {
	//	return nil, fmt.Errorf("error serializing lock info: %v", err)
	//}

	f, err := os.OpenFile(l.p, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return nil, fmt.Errorf("error opening lock file %q: %v", l.p, err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if syserr, ok := err.(syscall.Errno); ok {
			if syserr == syscall.EWOULDBLOCK {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("unexpected result from flock(%s, LOCK_EX): %v", l.p, err)
	}

	glog.Infof("Acquired FSLock on %s for %s", l.p, id)

	//if _, err := f.WriteAt(b, 0); err != nil {
	//	f.Close()
	//	return nil, fmt.Errorf("unexpected result from flock(%s, LOCK_EX): %v", l.p, ret)
	//}

	return &FSFlockLockGuard{
		p:    l.p,
		file: f,
	}, nil
}

func (l *FSFlockLockGuard) Release() error {
	ret := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	if ret != nil {
		l.file.Close()
		return fmt.Errorf("unexpected result from flock(%s, LOCK_UN): %v", l.p, ret)
	}

	if err := l.file.Close(); err != nil {
		return fmt.Errorf("unexpected result from closing lock file %s: %v", l.p, ret)
	}

	return nil
}
