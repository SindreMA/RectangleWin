package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	WH_KEYBOARD_LL = 13
	WM_KEYDOWN     = 0x0100
	WM_SYSKEYDOWN  = 0x0104
	VK_LWIN        = 0x5B
	VK_RWIN        = 0x5C
	VK_LCONTROL    = 0xA2
	VK_RCONTROL    = 0xA3
	VK_LSHIFT      = 0xA0
	VK_RSHIFT      = 0xA1
	VK_LMENU       = 0xA4
	VK_RMENU       = 0xA5
)

type KBDLLHOOKSTRUCT struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

var (
	procSetWindowsHookEx    = user32Hook.NewProc("SetWindowsHookExW")
	procCallNextHookEx      = user32Hook.NewProc("CallNextHookEx")
	procGetAsyncKeyState    = user32Hook.NewProc("GetAsyncKeyState")
	user32Hook              = syscall.NewLazyDLL("user32.dll")
	llHookHandle            uintptr
	llHookedKeys            []HotKey
)

func isKeyDown(vk uintptr) bool {
	r, _, _ := procGetAsyncKeyState.Call(vk)
	return r&0x8000 != 0
}

func llKeyboardProc(nCode int, wParam uintptr, lParam uintptr) uintptr {
	if nCode >= 0 && (wParam == WM_KEYDOWN || wParam == WM_SYSKEYDOWN) {
		kb := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))

		ctrlDown := isKeyDown(VK_LCONTROL) || isKeyDown(VK_RCONTROL)
		altDown := isKeyDown(VK_LMENU) || isKeyDown(VK_RMENU)
		shiftDown := isKeyDown(VK_LSHIFT) || isKeyDown(VK_RSHIFT)
		winDown := isKeyDown(VK_LWIN) || isKeyDown(VK_RWIN)

		for _, hk := range llHookedKeys {
			mod := hk.mod &^ MOD_NOREPEAT // strip NOREPEAT flag for comparison

			wantCtrl := mod&MOD_CONTROL != 0
			wantAlt := mod&MOD_ALT != 0
			wantShift := mod&MOD_SHIFT != 0
			wantWin := mod&MOD_WIN != 0

			if ctrlDown == wantCtrl && altDown == wantAlt && shiftDown == wantShift && winDown == wantWin && int(kb.VkCode) == hk.vk {
				fmt.Printf("trace: ll-hook matched id=%d (%s)\n", hk.id, hk)
				go hk.callback()
				return 1 // swallow the key
			}
		}
	}

	r, _, _ := procCallNextHookEx.Call(llHookHandle, uintptr(nCode), wParam, lParam)
	return r
}

func installLLHook(failedKeys []HotKey) {
	if len(failedKeys) == 0 {
		return
	}
	llHookedKeys = failedKeys

	cb := syscall.NewCallback(llKeyboardProc)
	r, _, err := procSetWindowsHookEx.Call(WH_KEYBOARD_LL, cb, 0, 0)
	if r == 0 {
		fmt.Printf("warn: SetWindowsHookEx failed: %v\n", err)
		return
	}
	llHookHandle = r
	fmt.Printf("low-level keyboard hook installed for %d hotkey(s)\n", len(failedKeys))
	for _, hk := range failedKeys {
		fmt.Printf("  - %s\n", hk.Describe())
	}
}
