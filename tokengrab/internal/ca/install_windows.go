//go:build windows

package ca

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Why we use crypt32 (and not registry writes):
//
// Windows 10/11 has an unsuppressible safety net: certificates added to the
// Root store by ANY method other than the certutil/CertAddCert UI prompt are
// silently ignored at chain validation time. We tested direct registry writes
// — the entry appears under HKCU\…\SystemCertificates\Root\Certificates, but
// Get-ChildItem Cert:\CurrentUser\Root cannot see it and Chromium reports
// ERR_CERT_AUTHORITY_INVALID.
//
// So we do what mkcert / Caddy / pretty much every dev MITM tool does: accept
// ONE Windows confirmation dialog on first install, then persist the CA so
// subsequent launches reuse it and never prompt again.

// InstallState carries the SHA-1 thumbprint of the cert in the store.
type InstallState struct {
	Thumbprint string // uppercase hex SHA-1 of the cert DER
}

const (
	storeProvSystem        uintptr = 10
	systemStoreCurrentUser uintptr = 1 << 16
	addReplaceExisting     uintptr = 3 // CERT_STORE_ADD_REPLACE_EXISTING
	findSHA1Hash           uintptr = 0x10000 | 2
	encodingX509ASN        uintptr = 1
	encodingPKCS7          uintptr = 0x10000
	encodingType                   = encodingX509ASN | encodingPKCS7
)

var (
	crypt32                              = windows.NewLazySystemDLL("crypt32.dll")
	procCertOpenStore                    = crypt32.NewProc("CertOpenStore")
	procCertCloseStore                   = crypt32.NewProc("CertCloseStore")
	procCertAddEncodedCertificateToStore = crypt32.NewProc("CertAddEncodedCertificateToStore")
	procCertFindCertificateInStore       = crypt32.NewProc("CertFindCertificateInStore")
	procCertDeleteCertificateFromStore   = crypt32.NewProc("CertDeleteCertificateFromStore")
	procCertFreeCertificateContext       = crypt32.NewProc("CertFreeCertificateContext")
)

type hashBlob struct {
	cbData uint32
	pbData *byte
}

func openRootStore() (uintptr, error) {
	name, err := windows.UTF16PtrFromString("Root")
	if err != nil {
		return 0, err
	}
	h, _, errno := procCertOpenStore.Call(
		storeProvSystem,
		0,
		0,
		systemStoreCurrentUser,
		uintptr(unsafe.Pointer(name)),
	)
	if h == 0 {
		return 0, fmt.Errorf("CertOpenStore: %v", errno)
	}
	return h, nil
}

// IsInstalled reports whether a cert with the given SHA-1 thumbprint is
// already in the user's Root store. Used to avoid re-prompting on every run.
func IsInstalled(thumbprint string) (bool, error) {
	if thumbprint == "" {
		return false, nil
	}
	target, err := hex.DecodeString(thumbprint)
	if err != nil {
		return false, err
	}
	h, err := openRootStore()
	if err != nil {
		return false, err
	}
	defer procCertCloseStore.Call(h, 0)

	blob := hashBlob{cbData: uint32(len(target)), pbData: &target[0]}
	pCert, _, _ := procCertFindCertificateInStore.Call(
		h, encodingType, 0, findSHA1Hash,
		uintptr(unsafe.Pointer(&blob)), 0,
	)
	if pCert == 0 {
		return false, nil
	}
	procCertFreeCertificateContext.Call(pCert)
	return true, nil
}

// Install adds the CA to the CurrentUser Root store. WILL trigger Windows'
// "Do you want to install this certificate?" dialog. Call only when
// IsInstalled returned false — i.e. once per machine.
func (c *CA) Install() (*InstallState, error) {
	h, err := openRootStore()
	if err != nil {
		return nil, err
	}
	defer procCertCloseStore.Call(h, 0)

	var pCert uintptr
	r, _, errno := procCertAddEncodedCertificateToStore.Call(
		h, encodingType,
		uintptr(unsafe.Pointer(&c.DER[0])),
		uintptr(len(c.DER)),
		addReplaceExisting,
		uintptr(unsafe.Pointer(&pCert)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CertAddEncodedCertificateToStore: %v", errno)
	}
	if pCert != 0 {
		procCertFreeCertificateContext.Call(pCert)
	}
	hash := sha1.Sum(c.DER)
	return &InstallState{Thumbprint: strings.ToUpper(hex.EncodeToString(hash[:]))}, nil
}

// Uninstall removes a cert from the user's Root store. Triggers Windows'
// removal-confirmation dialog. Only called when the user explicitly clicks
// "卸载持久 CA"; we never auto-uninstall.
func Uninstall(st *InstallState) error {
	if st == nil || st.Thumbprint == "" {
		return nil
	}
	target, err := hex.DecodeString(st.Thumbprint)
	if err != nil {
		return err
	}
	h, err := openRootStore()
	if err != nil {
		return err
	}
	defer procCertCloseStore.Call(h, 0)

	blob := hashBlob{cbData: uint32(len(target)), pbData: &target[0]}
	pCert, _, _ := procCertFindCertificateInStore.Call(
		h, encodingType, 0, findSHA1Hash,
		uintptr(unsafe.Pointer(&blob)), 0,
	)
	if pCert == 0 {
		return nil
	}
	r, _, errno := procCertDeleteCertificateFromStore.Call(pCert)
	if r == 0 {
		procCertFreeCertificateContext.Call(pCert)
		return fmt.Errorf("CertDeleteCertificateFromStore: %v", errno)
	}
	return nil
}
