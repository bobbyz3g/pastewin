//go:build windows

package native

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"pastewin/internal/clipimage"

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
	foreground atomic.Uint64 // clipboard sequence (high 32 bits), event-time PID (low 32 bits)
	handle     uintptr
	done       chan struct{}
	Changes    chan struct{}
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
				seq := Sequence()
				foreground, _, _ := user32.NewProc("GetForegroundWindow").Call()
				pid := windowPID(foreground)
				if Sequence() == seq {
					m.foreground.Store(uint64(seq)<<32 | uint64(pid))
				}
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

type Snapshot struct {
	SourcePID uint32
	Sequence  uint32
	Source    string
	Text      string
	Images    []clipimage.Encoded
	Warning   error
}

// ReadSnapshot copies bounded representations while holding the clipboard. No
// image decoding or encoding occurs until after CloseClipboard.
func (m *Monitor) ReadSnapshot() (result Snapshot, err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := open(m.handle); err != nil {
		return result, err
	}
	owner, _, _ := user32.NewProc("GetClipboardOwner").Call()
	pid := windowPID(owner)
	sourceMethod := "owner"
	defer func() {
		closeClipboard.Call()
		if result.Text != "" || len(result.Images) > 0 {
			result.SourcePID = pid
			result.Source = processPath(pid)
			slog.Info("clipboard_source", "sequence", result.Sequence, "owner_present", owner != 0, "pid", pid, "method", sourceMethod, "source_app", filepath.Base(result.Source), "source_path", result.Source, "identified", result.Source != "")
		}
	}()
	result.Sequence = Sequence()
	if owner == 0 {
		pid = m.fallbackSourcePID(result.Sequence)
		sourceMethod = "foreground_fallback"
	}
	// Respect Windows' explicit clipboard-history opt-out formats.
	if available(format("ExcludeClipboardContentFromMonitorProcessing")) {
		return result, nil
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
				return result, nil
			}
		}
	}
	result.Text, result.Warning = readText()
	var imageErr error
	remaining := clipimage.MaxRawBytes
	for _, f := range []struct {
		id   uintptr
		name string
	}{{format("PNG"), "PNG"}, {17, "DIB"}, {8, "DIB"}} {
		if f.id == 0 || !available(f.id) {
			continue
		}
		data, err := readBytes(f.id, remaining)
		if err != nil {
			imageErr = err
			continue
		}
		remaining -= len(data)
		result.Images = append(result.Images, clipimage.Encoded{Format: f.name, Data: data})
	}
	if len(result.Images) == 0 {
		result.Warning = errors.Join(result.Warning, imageErr)
	}
	return result, nil
}

func readBytes(f uintptr, limit int) ([]byte, error) {
	h, _, err := getClipboardData.Call(f)
	if h == 0 {
		return nil, fmt.Errorf("读取图片剪贴板：%w", err)
	}
	size, _, _ := globalSize.Call(h)
	if size == 0 || size > uintptr(limit) {
		return nil, errors.New("图片剪贴板数据过大或为空，未读取该格式")
	}
	p, _, err := globalLock.Call(h)
	if p == 0 {
		return nil, fmt.Errorf("锁定图片剪贴板：%w", err)
	}
	defer globalUnlock.Call(h)
	data := make([]byte, int(size))
	copyMemory.Call(uintptr(unsafe.Pointer(&data[0])), p, size)
	return data, nil
}

func readText() (string, error) {
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
	text := windows.UTF16ToString(data)
	if len(text) > 1<<20 {
		return "", errors.New("文本超过 1 MiB，未保存")
	}
	return text, nil
}

func (m *Monitor) WriteText(text string) (uint32, error) {
	data, err := windows.UTF16FromString(text)
	if err != nil {
		return 0, err
	}
	encoded := make([]byte, len(data)*2)
	for i, v := range data {
		binary.LittleEndian.PutUint16(encoded[i*2:], v)
	}
	return m.writeFormats([]clipboardFormat{{13, encoded}})
}

func (m *Monitor) WriteImage(pngData, dib []byte) (uint32, error) {
	f := format("PNG")
	if f == 0 {
		return 0, errors.New("无法注册 PNG 剪贴板格式")
	}
	return m.writeFormats([]clipboardFormat{{f, pngData}, {17, dib}})
}

type clipboardFormat struct {
	id   uintptr
	data []byte
}

func (m *Monitor) writeFormats(formats []clipboardFormat) (seq uint32, err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	handles := make([]uintptr, len(formats))
	defer func() {
		for _, h := range handles {
			if h != 0 {
				globalFree.Call(h)
			}
		}
	}()
	// Allocate every representation before emptying the clipboard. A successful
	// SetClipboardData transfers only that handle's ownership to Windows.
	for i, f := range formats {
		if len(f.data) == 0 {
			return 0, errors.New("剪贴板内容为空")
		}
		h, _, e := globalAlloc.Call(0x0042, uintptr(len(f.data)))
		if h == 0 {
			return 0, e
		}
		handles[i] = h
		p, _, e := globalLock.Call(h)
		if p == 0 {
			return 0, e
		}
		copyMemory.Call(p, uintptr(unsafe.Pointer(&f.data[0])), uintptr(len(f.data)))
		globalUnlock.Call(h)
	}
	if err := open(m.handle); err != nil {
		return 0, err
	}
	changed := false
	defer func() {
		closeClipboard.Call()
		// Closing a bitmap clipboard can synthesize formats and advance its
		// sequence again. Record the final sequence only while we still own it.
		if changed {
			current := Sequence()
			owner, _, _ := user32.NewProc("GetClipboardOwner").Call()
			if owner == m.handle {
				seq = current
			}
		}
	}()
	if ok, _, err := emptyClipboard.Call(); ok == 0 {
		return 0, err
	}
	// Also suppress a partial write: it has already changed the clipboard even
	// when a later representation fails. Do not send Ctrl+V on that error.
	changed = true
	for i, f := range formats {
		if ok, _, e := setClipboardData.Call(f.id, handles[i]); ok == 0 {
			return 0, fmt.Errorf("写入剪贴板失败：%w", e)
		}
		handles[i] = 0
	}
	return 0, nil
}
