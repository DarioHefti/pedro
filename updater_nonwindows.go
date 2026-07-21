//go:build !windows

package main

func (u *Updater) GetWindowsDefenderExclusion() string {
	return ""
}
