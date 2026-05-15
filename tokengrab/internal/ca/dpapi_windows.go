//go:build windows

package ca

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DPAPI (Data Protection API) lets us encrypt the CA private key with a key
// derived from the current user's login credentials. Cipher text is bound to
// this Windows user account on this machine — it can't be decrypted by other
// users, by an offline attacker without the user's password, or on another
// computer. This is the same mechanism Chrome / Edge use to encrypt saved
// passwords on Windows.
//
// We don't need a separate secret to manage. The user just runs tokengrab.exe
// as themselves, and the OS handles key derivation transparently.

var (
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
)

// dataBlob mirrors Windows' DATA_BLOB struct used by CryptProtectData /
// CryptUnprotectData. Independent of hashBlob above; same layout but used in
// a different API context, so we keep the types separate for clarity.
type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newDataBlob(data []byte) *dataBlob {
	if len(data) == 0 {
		return &dataBlob{}
	}
	return &dataBlob{cbData: uint32(len(data)), pbData: &data[0]}
}

// copyAndFree copies the bytes out of a Windows-allocated DATA_BLOB into a
// Go-owned slice, then LocalFree's the source.
func (b *dataBlob) copyAndFree() []byte {
	if b.cbData == 0 || b.pbData == nil {
		return nil
	}
	out := make([]byte, b.cbData)
	src := unsafe.Slice(b.pbData, b.cbData)
	copy(out, src)
	windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(b.pbData))))
	return out
}

func dpapiEncrypt(plain []byte) ([]byte, error) {
	in := newDataBlob(plain)
	var out dataBlob
	r, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CryptProtectData: %v", err)
	}
	return out.copyAndFree(), nil
}

func dpapiDecrypt(enc []byte) ([]byte, error) {
	in := newDataBlob(enc)
	var out dataBlob
	r, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CryptUnprotectData: %v", err)
	}
	return out.copyAndFree(), nil
}
