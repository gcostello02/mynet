package daemon

import (
	"fmt"
	"os"
	"os/user"
	"strconv"

	"github.com/gcostello02/mynet/internal/config"
)

// ChownToConfiguredOwner assigns path to MYNET_OWNER_USER (or SUDO_USER) when the daemon runs as root.
func ChownToConfiguredOwner(path string) error {
	if os.Geteuid() != 0 {
		return nil
	}
	if err := refuseSymlinkForChown(path); err != nil {
		return err
	}
	owner := config.OwnerUsername()
	if owner == "" || owner == "root" {
		if config.Supervised() {
			return fmt.Errorf("%s must be set for supervised IPC (got owner %q)", config.EnvOwnerUser, owner)
		}
		return nil
	}
	ou, err := user.Lookup(owner)
	if err != nil {
		return fmt.Errorf("owner user %q: %w", owner, err)
	}
	uid, err := strconv.Atoi(ou.Uid)
	if err != nil {
		return fmt.Errorf("owner uid: %w", err)
	}
	gid, err := strconv.Atoi(ou.Gid)
	if err != nil {
		return fmt.Errorf("owner gid: %w", err)
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("chown %s to %s: %w", path, owner, err)
	}
	return nil
}

func refuseSymlinkForChown(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to chown symlink: %s", path)
	}
	return nil
}
