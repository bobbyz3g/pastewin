//go:build windows

package native

import (
	"errors"
	"os"
	"time"
	"unsafe"
)

func Foreground() uintptr { h, _, _ := user32.NewProc("GetForegroundWindow").Call(); return h }
func ExternalWindow(h uintptr) bool {
	if h == 0 {
		return false
	}
	valid, _, _ := user32.NewProc("IsWindow").Call(h)
	if valid == 0 {
		return false
	}
	var pid uint32
	user32.NewProc("GetWindowThreadProcessId").Call(h, uintptr(unsafe.Pointer(&pid)))
	return pid != 0 && pid != uint32(os.Getpid())
}

// INPUT is 40 bytes on Windows amd64/arm64. The union must be pointer aligned.
type keyboardInput struct {
	Type        uint32
	Padding     uint32
	VK, Scan    uint16
	Flags, Time uint32
	Extra       uintptr
	Tail        [8]byte
}

func Paste(ctxDone <-chan struct{}, target uintptr) error {
	if !ExternalWindow(target) {
		return errors.New("请先在目标应用中按 Ctrl+Shift+V 打开面板，再选择内容")
	}
	deadline := time.Now().Add(1200 * time.Millisecond)
	for {
		pressed := false
		for _, key := range []uintptr{0x10, 0x11, 0x12, 0x5B, 0x5C, 0x0D} {
			r, _, _ := user32.NewProc("GetAsyncKeyState").Call(key)
			if r&0x8000 != 0 {
				pressed = true
			}
		}
		if !pressed {
			break
		}
		if time.Now().After(deadline) {
			return errors.New("请释放快捷键后重试")
		}
		select {
		case <-ctxDone:
			return errors.New("程序正在退出")
		case <-time.After(10 * time.Millisecond):
		}
	}
	user32.NewProc("SetForegroundWindow").Call(target)
	deadline = time.Now().Add(350 * time.Millisecond)
	for Foreground() != target {
		if time.Now().After(deadline) {
			return errors.New("无法激活目标窗口；内容已复制，可手动按 Ctrl+V")
		}
		time.Sleep(10 * time.Millisecond)
	}
	inputs := []keyboardInput{{Type: 1, VK: 0x11}, {Type: 1, VK: 0x56}, {Type: 1, VK: 0x56, Flags: 2}, {Type: 1, VK: 0x11, Flags: 2}}
	n, _, _ := user32.NewProc("SendInput").Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(inputs[0]))
	if n != 4 {
		// Release injected keys even if Windows accepted only part of the sequence.
		up := inputs[2:]
		user32.NewProc("SendInput").Call(2, uintptr(unsafe.Pointer(&up[0])), unsafe.Sizeof(up[0]))
		return errors.New("Windows 未接受粘贴按键，目标可能以管理员权限运行；内容已复制")
	}
	return nil
}
