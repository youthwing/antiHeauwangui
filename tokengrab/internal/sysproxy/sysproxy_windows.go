//go:build windows

// Package sysproxy reads / writes / restores the per-user Windows system proxy
// settings (the same registry keys Internet Explorer / Edge / Chromium read).
package sysproxy

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const proxyKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// State captures the original settings so we can roll back on shutdown.
type State struct {
	Enable uint32 // 0 = off, 1 = on
	Server string // e.g. "127.0.0.1:8888" or ""
}

// Snapshot reads the current state before we modify it.
func Snapshot() (*State, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, proxyKey, registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("open InternetSettings: %w", err)
	}
	defer k.Close()

	st := &State{}
	if enable, _, err := k.GetIntegerValue("ProxyEnable"); err == nil {
		st.Enable = uint32(enable)
	}
	if server, _, err := k.GetStringValue("ProxyServer"); err == nil {
		st.Server = server
	}
	return st, nil
}

// Set forces ProxyEnable=1 and ProxyServer=server, then notifies WinINet.
func Set(server string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, proxyKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open InternetSettings: %w", err)
	}
	defer k.Close()
	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return err
	}
	if err := k.SetStringValue("ProxyServer", server); err != nil {
		return err
	}
	notify()
	return nil
}

// Restore writes the previously captured state back.
func Restore(st *State) error {
	if st == nil {
		return nil
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, proxyKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open InternetSettings: %w", err)
	}
	defer k.Close()
	if err := k.SetDWordValue("ProxyEnable", st.Enable); err != nil {
		return err
	}
	if err := k.SetStringValue("ProxyServer", st.Server); err != nil {
		return err
	}
	notify()
	return nil
}

// IsArmedToLocalhost returns true if the current proxy settings still point at
// 127.0.0.1 — used at startup to detect a previous crashed session.
func IsArmedToLocalhost() bool {
	st, err := Snapshot()
	if err != nil {
		return false
	}
	if st.Enable != 1 {
		return false
	}
	return len(st.Server) > len("127.0.0.1:") && st.Server[:10] == "127.0.0.1:"
}

// notify tells WinINet to reload the settings immediately.
const (
	internetOptionSettingsChanged uintptr = 39
	internetOptionRefresh         uintptr = 37
)

var (
	wininet                = syscall.NewLazyDLL("wininet.dll")
	procInternetSetOptionW = wininet.NewProc("InternetSetOptionW")
)

func notify() {
	procInternetSetOptionW.Call(0, internetOptionSettingsChanged, 0, 0)
	procInternetSetOptionW.Call(0, internetOptionRefresh, 0, 0)
}
