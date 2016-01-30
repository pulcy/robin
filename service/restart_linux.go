package service

import (
	"os/exec"
	"syscall"
)

func configureRestartHaproxyCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGUSR1}
}
