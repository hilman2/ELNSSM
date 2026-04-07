package process

import (
	"encoding/base64"
	"fmt"
	"math"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modCrypt32             = windows.NewLazySystemDLL("crypt32.dll")
	procCryptProtectData   = modCrypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = modCrypt32.NewProc("CryptUnprotectData")

	modKernel32ForCred = windows.NewLazySystemDLL("kernel32.dll")
	procLocalFree      = modKernel32ForCred.NewProc("LocalFree")
)

// CRYPTPROTECT_LOCAL_MACHINE makes the encrypted data machine-bound.
const cryptprotectLocalMachine = 0x04

// dataBlob mirrors the Windows DATA_BLOB structure.
type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newDataBlob(data []byte) *dataBlob {
	if len(data) == 0 {
		return &dataBlob{}
	}
	// DPAPI blobs are at most a few KB; len(data) cannot overflow uint32 in practice.
	if len(data) > math.MaxUint32 {
		return &dataBlob{}
	}
	return &dataBlob{
		cbData: uint32(len(data)), //nolint:gosec // bounded above
		pbData: &data[0],
	}
}

func (b *dataBlob) toBytes() []byte {
	if b.cbData == 0 || b.pbData == nil {
		return nil
	}
	return unsafe.Slice(b.pbData, b.cbData) //nolint:gosec // Win32 DPAPI binding
}

func (b *dataBlob) free() {
	if b.pbData != nil {
		_, _, _ = procLocalFree.Call(uintptr(unsafe.Pointer(b.pbData))) //nolint:gosec // Win32 API binding
	}
}

// EncryptPassword encrypts a plaintext password using Windows DPAPI.
// The result is a base64-encoded blob that is machine-bound.
func EncryptPassword(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	input := []byte(plaintext)
	inputBlob := newDataBlob(input)
	var outputBlob dataBlob

	ret, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(inputBlob)), //nolint:gosec // Win32 API binding
		0,                                  // description
		0,                                  // optional entropy
		0,                                  // reserved
		0,                                  // prompt struct
		uintptr(cryptprotectLocalMachine),
		uintptr(unsafe.Pointer(&outputBlob)), //nolint:gosec // Win32 API binding
	)
	if ret == 0 {
		return "", fmt.Errorf("CryptProtectData failed: %w", err)
	}
	defer outputBlob.free()

	encrypted := make([]byte, outputBlob.cbData)
	copy(encrypted, outputBlob.toBytes())

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// DecryptPassword decrypts a DPAPI-encrypted base64-encoded password.
func DecryptPassword(encrypted string) (string, error) {
	if encrypted == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("invalid base64: %w", err)
	}

	inputBlob := newDataBlob(data)
	var outputBlob dataBlob

	ret, _, callErr := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(inputBlob)), //nolint:gosec // Win32 API binding
		0,                                  // description
		0,                                  // optional entropy
		0,                                  // reserved
		0,                                  // prompt struct
		uintptr(cryptprotectLocalMachine),
		uintptr(unsafe.Pointer(&outputBlob)), //nolint:gosec // Win32 API binding
	)
	if ret == 0 {
		return "", fmt.Errorf("CryptUnprotectData failed: %w", callErr)
	}
	defer outputBlob.free()

	plaintext := make([]byte, outputBlob.cbData)
	copy(plaintext, outputBlob.toBytes())

	return string(plaintext), nil
}
