package util

import (
	"os"
	"os/signal"
	"syscall"

	"go.jonnrb.io/egress/log"
)

func ReapChildren(child *os.Process) error {
	// forward all signals to child
	c := make(chan os.Signal)
	defer close(c)
	signal.Notify(c)
	defer signal.Stop(c)
	go func() {
		for sig := range c {
			child.Signal(sig)
		}
	}()

	log.V(2).Infof("waiting for child %v to exit; forwarding all signals", child.Pid)

	var wstatus syscall.WaitStatus
	var err error
	for pid := -1; pid != child.Pid; {
		pid, err = syscall.Wait4(-1, &wstatus, 0, nil)
		if err != nil {
			return err
		}
		log.Infof("reaped pid %v", pid)
	}

	if exitCode := wstatus.ExitStatus(); exitCode != 0 {
		log.Errorf("child exited with code %v", exitCode)
	} else {
		log.V(2).Infof("child exited with code %v", exitCode)
	}

	return nil
}
