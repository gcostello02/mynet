//go:build !darwin

package service

import "fmt"

func InstallDarwinSystemBinary(source string) error {
	return fmt.Errorf("system binary install is only supported on macOS")
}

func FinalizeDarwinSystemBinary(path string) error {
	return fmt.Errorf("system binary finalize is only supported on macOS")
}
