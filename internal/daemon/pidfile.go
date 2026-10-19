package daemon

import (
	"fmt"
	"os"
)

// WritePIDFile records the current process id and assigns ownership for CLI access.
func WritePIDFile(stateDir string) error {
	path := PIDPath(stateDir)
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o600); err != nil {
		return err
	}
	return ChownToConfiguredOwner(path)
}
