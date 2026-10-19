package service

import "os"

// localSpecPresence reports whether install metadata exists and is readable.
func localSpecPresence(path string) (present, readable bool, err error) {
	if _, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			return false, false, nil
		}
		return false, false, statErr
	}
	_, _, loadErr := LoadLocalSpec(path)
	if loadErr != nil {
		if os.IsPermission(loadErr) {
			return true, false, nil
		}
		return true, false, loadErr
	}
	return true, true, nil
}
