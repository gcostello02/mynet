//go:build !linux

package service

func probeSupervisor(layout SystemLayout) (active, enabledAtBoot bool, activeKnown, enabledKnown bool) {
	return false, false, false, false
}
