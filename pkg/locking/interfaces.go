package locking

import "context"

// Lock is the interface for a mutex
type Lock interface {
	Acquire(ctx context.Context, id string) (LockGuard, error)
}

// LockGuard is a lock that is actually held
type LockGuard interface {
	Release() error
}
