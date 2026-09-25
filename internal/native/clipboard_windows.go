//go:build windows

package native

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var user32 = windows.NewLazySystemDLL("user32.dll")
var kernel32 = windows.NewLazySystemDLL("kernel32.dll")
var openClipboard = user32.NewProc("OpenClipboard")
var closeClipboard = user32.NewProc("CloseClipboard")
var getClipboardData = user32.NewProc("GetClipboardData")
var setClipboardData = user32.NewProc("SetClipboardData")
var emptyClipboard = user32.NewProc("EmptyClipboard")
var hasFormat = user32.NewProc("IsClipboardFormatAvailable")
var sequence = user32.NewProc("GetClipboardSequenceNumber")
var globalLock = kernel32.NewProc("GlobalLock")
var globalUnlock = kernel32.NewProc("GlobalUnlock")
var globalSize = kernel32.NewProc("GlobalSize")
var globalAlloc = kernel32.NewProc("GlobalAlloc")
var globalFree = kernel32.NewProc("GlobalFree")
var copyMemory = kernel32.NewProc("RtlMoveMemory")

type point struct{ X, Y int32 }
type message struct {
	Window         uintptr
	ID             uint32
	WParam, LParam uintptr
	Time           uint32
	Point          point
	Private        uint32
}
type windowClass struct {
	Size, Style                                               uint32
	Proc                                                      uintptr
	ClassExtra, WindowExtra                                   int32
	Instance, Icon, Cursor, Background, Menu, Name, SmallIcon uintptr
}

type Monitor struct {
	handle  uintptr
	done    chan struct{}
	Changes chan struct{}
}

// The listener owns a message-only window on one OS thread. No polling while idle.
func Watch(ctx context.Context) (*Monitor, error) {
	m := &Monitor{Changes: make(chan struct{}, 1), done: make(chan struct{})}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(m.done)
		def := user32.NewProc("DefWindowProcW")
		destroy := user32.NewProc("DestroyWindow")
		quit := user32.NewProc("PostQuitMessage")
		proc := syscall.NewCallback(func(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
			switch msg {
			case 0x031D:
				select {
				case m.Changes <- struct{}{}:
				default:
				}
				return 0
			case 0x0010:
				destroy.Call(hwnd)
				return 0
			case 0x0002:
				quit.Call(0)
				return 0
			}
			result, _, _ := def.Call(hwnd, uintptr(msg), wp, lp)
			return result
		})
		name, _ := windows.UTF16PtrFromString("PasteWinClipboardListener")
		instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
		wc := windowClass{Proc: proc, Instance: instance, Name: uintptr(unsafe.Pointer(name))}
		wc.Size = uint32(unsafe.Sizeof(wc))
		atom, _, err := user32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc)))
		if atom == 0 {
			ready <- fmt.Errorf("注册监听窗口: %w", err)
			return
		}
		defer user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(name)), instance)
		hwnd, _, err := user32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(name)), 0, 0, 0, 0, 0, 0, ^uintptr(2), 0, instance, 0)
		if hwnd == 0 {
			ready <- fmt.Errorf("创建监听窗口: %w", err)
			return
		}
		m.handle = hwnd
		ok, _, err := user32.NewProc("AddClipboardFormatListener").Call(hwnd)
		if ok == 0 {
			destroy.Call(hwnd)
			ready <- fmt.Errorf("监听剪贴板: %w", err)
			return
		}
		defer user32.NewProc("RemoveClipboardFormatListener").Call(hwnd)
		ready <- nil
		go func() {
			select {
			case <-ctx.Done():
				user32.NewProc("PostMessageW").Call(hwnd, 0x0010, 0, 0)
			case <-m.done:
			}
		}()
		var msg message
		for {
			r, _, _ := user32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(r) <= 0 {
				break
			}
			user32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&msg)))
			user32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	return m, nil
}
func (m *Monitor) Wait() { <-m.done }
func Sequence() uint32   { r, _, _ := sequence.Call(); return uint32(r) }

func open(owner uintptr) error {
	for attempt := 0; attempt < 8; attempt++ {
		r, _, _ := openClipboard.Call(owner)
		if r != 0 {
			return nil
		}
		time.Sleep(time.Duration(attempt+1) * 8 * time.Millisecond)
	}
	return errors.New("剪贴板正被其他程序占用，请重试")
}

func format(name string) uintptr {
	p, _ := windows.UTF16PtrFromString(name)
	r, _, _ := user32.NewProc("RegisterClipboardFormatW").Call(uintptr(unsafe.Pointer(p)))
	return r
}
func available(f uintptr) bool { r, _, _ := hasFormat.Call(f); return r != 0 }

func (m *Monitor) ReadText() (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := open(m.handle); err != nil {
		return "", err
	}
	defer closeClipboard.Call()
	// Respect Windows' explicit clipboard-history opt-out formats.
	if available(format("ExcludeClipboardContentFromMonitorProcessing")) {
		return "", nil
	}
	if f := format("CanIncludeInClipboardHistory"); available(f) {
		h, _, _ := getClipboardData.Call(f)
		size, _, _ := globalSize.Call(h)
		p, _, _ := globalLock.Call(h)
		if p != 0 {
			var flag uint32
			if size >= 4 {
				copyMemory.Call(uintptr(unsafe.Pointer(&flag)), p, 4)
			}
			excluded := size >= 4 && flag == 0
			globalUnlock.Call(h)
			if excluded {
				return "", nil
			}
		}
	}
	if !available(13) {
		return "", nil
	}
	h, _, _ := getClipboardData.Call(13)
	if h == 0 {
		return "", nil
	}
	size, _, _ := globalSize.Call(h)
	if size > 2*(1<<20)+2 {
		return "", errors.New("文本超过 1 MB，未保存")
	}
	if size < 2 {
		return "", nil
	}
	p, _, err := globalLock.Call(h)
	if p == 0 {
		return "", fmt.Errorf("读取剪贴板: %w", err)
	}
	defer globalUnlock.Call(h)
	// Copy only the bounded, locked Windows allocation into Go-owned memory.
	data := make([]uint16, int(size/2))
	copyMemory.Call(uintptr(unsafe.Pointer(&data[0])), p, uintptr(len(data)*2))
	return windows.UTF16ToString(data), nil
}

func (m *Monitor) WriteText(text string) (uint32, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	data, err := windows.UTF16FromString(text)
	if err != nil {
		return 0, err
	}
	h, _, err := globalAlloc.Call(0x0042, uintptr(len(data)*2))
	if h == 0 {
		return 0, err
	}
	owned := true
	defer func() {
		if owned {
			globalFree.Call(h)
		}
	}()
	p, _, err := globalLock.Call(h)
	if p == 0 {
		return 0, err
	}
	copyMemory.Call(p, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)*2))
	globalUnlock.Call(h)
	if err := open(m.handle); err != nil {
		return 0, err
	}
	defer closeClipboard.Call()
	if ok, _, err := emptyClipboard.Call(); ok == 0 {
		return 0, err
	}
	if ok, _, err := setClipboardData.Call(13, h); ok == 0 {
		return 0, err
	}
	owned = false
	return Sequence(), nil
}
