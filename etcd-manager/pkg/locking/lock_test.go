package locking

import (
	"context"
	"flag"
	"io/ioutil"
	"path/filepath"
	"testing"

	"k8s.io/klog"
)

func init() {
	logflags := flag.NewFlagSet("testing", flag.ExitOnError)

	klog.InitFlags(logflags)

	logflags.Set("logtostderr", "true")
	logflags.Set("v", "2")
	logflags.Parse([]string{})
}

func checkLocks(t *testing.T, l1, l2 Lock) {
	ctx := context.TODO()

	// Should be able to lock the new lock
	lg1, err := l1.Acquire(ctx, "1")
	if err != nil {
		t.Fatalf("unable to acquire new lock: %v", err)
	}
	if lg1 == nil {
		t.Fatalf("lock was unexpected nil")
	}

	// Second process cannot acquire it
	lg2, err := l2.Acquire(ctx, "2")
	if lg2 != nil {
		t.Fatalf("able to double-lock lock: %v", err)
	}
	if err != nil {
		t.Fatalf("expected no error when failed to aquire lock: %v", err)
	}

	// Release first lock
	if err := lg1.Release(); err != nil {
		t.Fatalf("unable to release lock")
	}

	// Second process can now acquire it
	lg2, err = l2.Acquire(ctx, "2")
	if err != nil {
		t.Fatalf("unable to acquire new lock: %v", err)
	}
	if lg2 == nil {
		t.Fatalf("lock was unexpected nil")
	}

	// First process cannot reacquire it
	lg1, err = l1.Acquire(ctx, "1")
	if lg1 != nil {
		t.Fatalf("able to double-lock lock: %v", err)
	}
	if err != nil {
		t.Fatalf("expected no error when failed to aquire lock: %v", err)
	}

	// Release second lock
	if err := lg2.Release(); err != nil {
		t.Fatalf("unable to release lock")
	}

	// First process can acquire it again
	lg1, err = l1.Acquire(ctx, "1")
	if err != nil {
		t.Fatalf("unable to acquire new lock: %v", err)
	}
	if lg1 == nil {
		t.Fatalf("lock was unexpected nil")
	}

	// Release lock
	if err := lg1.Release(); err != nil {
		t.Fatalf("unable to release lock")
	}
}

func TestFSFlockLock(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Errorf("error building tempdir: %v", err)
	}

	p := filepath.Join(tmpDir, "lock")
	l1, err := NewFSFlockLock(p)
	if err != nil {
		t.Fatalf("error building lock: %v", err)
	}
	l2, err := NewFSFlockLock(p)
	if err != nil {
		t.Fatalf("error building lock: %v", err)
	}

	checkLocks(t, l1, l2)
}

func TestFSContentLock(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Errorf("error building tempdir: %v", err)
	}

	p := filepath.Join(tmpDir, "lock")
	l1, err := NewFSContentLock(p)
	if err != nil {
		t.Fatalf("error building lock: %v", err)
	}
	l2, err := NewFSContentLock(p)
	if err != nil {
		t.Fatalf("error building lock: %v", err)
	}

	checkLocks(t, l1, l2)
}
