//go:build windows

// A clipboard-free fixture for the Wails hidden-window shutdown contract.
package main

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

func main() {
	runtime.LockOSThread()
	user := syscall.NewLazyDLL("user32.dll")
	kernel := syscall.NewLazyDLL("kernel32.dll")
	instance, _, _ := kernel.NewProc("GetModuleHandleW").Call(0)
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	class, _ := syscall.UTF16PtrFromString("WailsWebviewWindow")
	title, _ := syscall.UTF16PtrFromString("__wails_hidden_mainthread")
	type windowClass struct {
		Size, Style                   uint32
		Proc                          uintptr
		ClassExtra, WindowExtra       int32
		Instance, Icon, Cursor, Brush uintptr
		Menu, Name                    *uint16
		SmallIcon                     uintptr
	}
	wc := windowClass{Instance: instance, Name: class}
	wc.Size = uint32(unsafe.Sizeof(wc))
	wc.Proc = syscall.NewCallback(func(hwnd uintptr, msg uint32, w, l uintptr) uintptr {
		if msg == 0x10 {
			if len(os.Args) > 1 && os.Args[1] == "--ignore-close" {
				return 0
			}
			_ = os.WriteFile(filepath.Join(dir, "closed"), []byte("graceful"), 0600)
			user.NewProc("PostQuitMessage").Call(0)
			return 0
		}
		result, _, _ := user.NewProc("DefWindowProcW").Call(hwnd, uintptr(msg), w, l)
		return result
	})
	atom, _, _ := user.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		os.Exit(2)
	}
	hwnd, _, _ := user.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), 0x08000000, 0, 0, 0, 0, 0, 0, instance, 0)
	if hwnd == 0 {
		os.Exit(3)
	}
	_ = os.WriteFile(filepath.Join(dir, "ready"), []byte("ready"), 0600)
	var message [48]byte // MSG on Windows 64-bit
	for {
		result, _, _ := user.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			break
		}
		user.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&message)))
		user.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&message)))
	}
}
