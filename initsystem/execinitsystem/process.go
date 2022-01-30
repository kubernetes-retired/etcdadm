package execinitsystem

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"k8s.io/klog/v2"
	"sigs.k8s.io/etcdadm/apis"
	"sigs.k8s.io/etcdadm/service"
)

type process struct {
	cmd *exec.Cmd

	mutex     sync.Mutex
	exitState *os.ProcessState
	exitError error
}

func startProcess(config *apis.EtcdAdmConfig) (*process, error) {
	c := exec.Command(config.EtcdExecutable)
	c.Dir = config.DataDir

	klog.Infof("executing command %s %s", c.Path, c.Args)

	env, err := service.BuildEnvironmentMap(config)
	if err != nil {
		return nil, err
	}
	for k, v := range env {
		c.Env = append(c.Env, k+"="+v)
	}

	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("error starting etcd: %w", err)
	}

	p := &process{
		cmd: c,
	}

	go func() {
		processState, err := p.cmd.Process.Wait()
		if err != nil {
			klog.Warningf("etcd exited with error: %v", err)
		}
		p.mutex.Lock()
		p.exitState = processState
		p.exitError = err
		p.mutex.Unlock()
		exitCode := -2
		if processState != nil {
			exitCode = processState.ExitCode()
		}
		klog.Infof("etcd process exited (datadir %s; pid=%d); exitCode=%d, exitErr=%v", config.DataDir, p.cmd.Process.Pid, exitCode, err)
	}()

	return p, nil
}

func (p *process) IsActive() (bool, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.exitState != nil, nil
}

func (p *process) Stop(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.cmd == nil {
		return nil
	}

	if p.exitState != nil {
		return nil
	}

	if err := p.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if p.exitState != nil {
			return nil
		}

		// Hack because we can't wait for a condition with a timeout
		p.mutex.Unlock()
		time.Sleep(100 * time.Millisecond)
		p.mutex.Lock()
	}
}
