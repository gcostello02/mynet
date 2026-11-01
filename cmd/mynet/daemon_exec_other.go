//go:build !unix

package main

import "os/exec"

func configureDaemonSubprocess(cmd *exec.Cmd) {}
