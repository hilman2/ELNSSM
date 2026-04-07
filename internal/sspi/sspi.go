package sspi

import (
	"fmt"
	"math"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Win32 SSPI bindings via secur32.dll for Negotiate/NTLM authentication.

var (
	modSecur32 = windows.NewLazySystemDLL("secur32.dll")

	procAcquireCredentialsHandleW = modSecur32.NewProc("AcquireCredentialsHandleW")
	procAcceptSecurityContext     = modSecur32.NewProc("AcceptSecurityContext")
	procQueryContextAttributesW   = modSecur32.NewProc("QueryContextAttributesW")
	procQuerySecurityContextToken = modSecur32.NewProc("QuerySecurityContextToken")
	procDeleteSecurityContext     = modSecur32.NewProc("DeleteSecurityContext")
	procFreeCredentialsHandle     = modSecur32.NewProc("FreeCredentialsHandle")
	procFreeContextBuffer         = modSecur32.NewProc("FreeContextBuffer")
)

// SecurityStatus represents an SSPI return code.
type SecurityStatus uint32

// Exported status values for use by callers.
const (
	StatusOK              SecurityStatus = 0x00000000
	StatusContinueNeeded  SecurityStatus = 0x00090312
	StatusCompleteNeeded  SecurityStatus = 0x00090313
	StatusCompleteAndCont SecurityStatus = 0x00090314
)

// IsError returns true if the status represents a failure.
func (s SecurityStatus) IsError() bool {
	return s&0x80000000 != 0
}

func (s SecurityStatus) Error() string {
	return fmt.Sprintf("SSPI error 0x%08X", uint32(s))
}

// AcceptSecurityContext flags.
const (
	ascReqDelegate        = 0x00000001
	ascReqMutualAuth      = 0x00000002
	ascReqReplayDetect    = 0x00000004
	ascReqSequenceDetect  = 0x00000008
	ascReqConfidentiality = 0x00000010
	ascReqAllocateMemory  = 0x00000100
	ascReqConnection      = 0x00000800
)

// SECPKG_ATTR constants.
const (
	secpkgAttrNames = 1
)

// SECBUFFER types.
const (
	secbufferToken   = 2
	secbufferVersion = 0
)

// SecHandle is a SSPI credential/context handle (two-pointer struct).
type SecHandle struct {
	Lower uintptr
	Upper uintptr
}

// IsZero returns true if the handle has not been initialized.
func (h *SecHandle) IsZero() bool {
	return h.Lower == 0 && h.Upper == 0
}

// TimeStamp is the SSPI SECURITY_INTEGER (TimeStamp) type.
type TimeStamp struct {
	LowPart  uint32
	HighPart int32
}

// SecBuffer is the SSPI SecBuffer struct.
type SecBuffer struct {
	Count  uint32
	Type   uint32
	Buffer *byte
}

// SecBufferDesc is the SSPI SecBufferDesc struct.
type SecBufferDesc struct {
	Version uint32
	Count   uint32
	Buffers *SecBuffer
}

// SecPkgContextNamesW is used with QueryContextAttributesW(SECPKG_ATTR_NAMES).
type SecPkgContextNamesW struct {
	UserName *uint16
}

// AcquireCredentialsHandle acquires a server credential handle for the "Negotiate" package.
func AcquireCredentialsHandle() (*SecHandle, error) {
	pkgName, err := windows.UTF16PtrFromString("Negotiate")
	if err != nil {
		return nil, fmt.Errorf("UTF16PtrFromString: %w", err)
	}

	var cred SecHandle
	var expiry TimeStamp

	const credentialInbound = 1

	// unsafe.Pointer is the standard mechanism for passing
	// Go addresses to Win32 SSPI functions.
	ret, _, _ := procAcquireCredentialsHandleW.Call(
		0,                                //nolint:gosec // principal
		uintptr(unsafe.Pointer(pkgName)), //nolint:gosec // Win32 API binding
		uintptr(credentialInbound),
		0,                                // logon ID
		0,                                // auth data
		0,                                // get key fn
		0,                                // get key arg
		uintptr(unsafe.Pointer(&cred)),   //nolint:gosec // Win32 API binding
		uintptr(unsafe.Pointer(&expiry)), //nolint:gosec // Win32 API binding
	)

	status := SecurityStatus(ret)
	if status.IsError() {
		return nil, fmt.Errorf("AcquireCredentialsHandle: %s", status.Error())
	}

	return &cred, nil
}

// AcceptSecurityContext processes a client token and produces a server response token.
// For the first call, pass nil for ctxIn. Returns the output token, the context handle,
// the SSPI status, and any error.
func AcceptSecurityContext(cred *SecHandle, ctxIn *SecHandle, inputToken []byte) ([]byte, *SecHandle, SecurityStatus, error) {
	// SSPI tokens are at most a few kilobytes; len(inputToken) cannot
	// exceed math.MaxUint32 in any realistic scenario.
	if len(inputToken) > math.MaxUint32 {
		return nil, nil, 0, fmt.Errorf("input token too large: %d bytes", len(inputToken))
	}
	// Set up input buffer
	inBuf := SecBuffer{
		Count:  uint32(len(inputToken)), //nolint:gosec // bounded above
		Type:   secbufferToken,
		Buffer: &inputToken[0],
	}
	inBufDesc := SecBufferDesc{
		Version: secbufferVersion,
		Count:   1,
		Buffers: &inBuf,
	}

	// Set up output buffer (allocate by SSPI)
	var outBuf SecBuffer
	outBuf.Type = secbufferToken
	outBufDesc := SecBufferDesc{
		Version: secbufferVersion,
		Count:   1,
		Buffers: &outBuf,
	}

	var ctxOut SecHandle
	var expiry TimeStamp
	var attrs uint32

	const contextFlags = ascReqConnection | ascReqAllocateMemory | ascReqReplayDetect | ascReqSequenceDetect

	var ctxInPtr uintptr
	if ctxIn != nil && !ctxIn.IsZero() {
		ctxInPtr = uintptr(unsafe.Pointer(ctxIn)) //nolint:gosec // Win32 API binding
	}

	// All unsafe.Pointer uses below are mandated by the Win32 SSPI ABI.
	ret, _, _ := procAcceptSecurityContext.Call(
		uintptr(unsafe.Pointer(cred)), //nolint:gosec // Win32 API binding
		ctxInPtr,
		uintptr(unsafe.Pointer(&inBufDesc)), //nolint:gosec // Win32 API binding
		uintptr(contextFlags),
		0,                                    // target data rep: SECURITY_NATIVE_DREP
		uintptr(unsafe.Pointer(&ctxOut)),     //nolint:gosec // Win32 API binding
		uintptr(unsafe.Pointer(&outBufDesc)), //nolint:gosec // Win32 API binding
		uintptr(unsafe.Pointer(&attrs)),      //nolint:gosec // Win32 API binding
		uintptr(unsafe.Pointer(&expiry)),     //nolint:gosec // Win32 API binding
	)

	status := SecurityStatus(ret)

	// Extract output token if present
	var outputToken []byte
	if outBuf.Count > 0 && outBuf.Buffer != nil {
		outputToken = make([]byte, outBuf.Count)
		copy(outputToken, unsafe.Slice(outBuf.Buffer, outBuf.Count))                 //nolint:gosec // Win32 API binding
		_, _, _ = procFreeContextBuffer.Call(uintptr(unsafe.Pointer(outBuf.Buffer))) //nolint:gosec // Win32 API binding
	}

	if status.IsError() {
		return nil, nil, status, fmt.Errorf("AcceptSecurityContext: %s", status.Error())
	}

	return outputToken, &ctxOut, status, nil
}

// QueryContextNames extracts the authenticated username (e.g. "DOMAIN\user") from a completed context.
func QueryContextNames(ctx *SecHandle) (string, error) {
	var names SecPkgContextNamesW

	ret, _, _ := procQueryContextAttributesW.Call(
		uintptr(unsafe.Pointer(ctx)), //nolint:gosec // Win32 API binding
		uintptr(secpkgAttrNames),
		uintptr(unsafe.Pointer(&names)), //nolint:gosec // Win32 API binding
	)

	status := SecurityStatus(ret)
	if status.IsError() {
		return "", fmt.Errorf("QueryContextAttributes(NAMES): %s", status.Error())
	}

	if names.UserName == nil {
		return "", fmt.Errorf("QueryContextAttributes returned nil username")
	}

	username := windows.UTF16PtrToString(names.UserName)
	_, _, _ = procFreeContextBuffer.Call(uintptr(unsafe.Pointer(names.UserName))) //nolint:gosec // Win32 API binding

	return username, nil
}

// QuerySecurityContextToken returns the impersonation token for the authenticated user.
func QuerySecurityContextToken(ctx *SecHandle) (windows.Token, error) {
	var token windows.Token

	ret, _, _ := procQuerySecurityContextToken.Call(
		uintptr(unsafe.Pointer(ctx)),    //nolint:gosec // Win32 API binding
		uintptr(unsafe.Pointer(&token)), //nolint:gosec // Win32 API binding
	)

	status := SecurityStatus(ret)
	if status.IsError() {
		return 0, fmt.Errorf("QuerySecurityContextToken: %s", status.Error())
	}

	return token, nil
}

// DeleteSecurityContext frees a security context handle.
func DeleteSecurityContext(ctx *SecHandle) {
	if ctx != nil && !ctx.IsZero() {
		_, _, _ = procDeleteSecurityContext.Call(uintptr(unsafe.Pointer(ctx))) //nolint:gosec // Win32 API binding
	}
}

// FreeCredentialsHandle frees a credential handle.
func FreeCredentialsHandle(cred *SecHandle) {
	if cred != nil && !cred.IsZero() {
		_, _, _ = procFreeCredentialsHandle.Call(uintptr(unsafe.Pointer(cred))) //nolint:gosec // Win32 API binding
	}
}

// IsAdmin checks whether the given token is a member of BUILTIN\Administrators.
func IsAdmin(token windows.Token) (bool, error) {
	sid, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return false, fmt.Errorf("CreateWellKnownSid: %w", err)
	}

	member, err := token.IsMember(sid)
	if err != nil {
		return false, fmt.Errorf("Token.IsMember: %w", err)
	}

	return member, nil
}
