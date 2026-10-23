//go:build windows

package native

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func windowPID(hwnd uintptr) uint32 {
	var pid uint32
	if hwnd != 0 {
		user32.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	}
	return pid
}

// Some applications publish with a NULL clipboard owner. Use the foreground
// observed at notification time only for that exact clipboard sequence, never
// the foreground at read time (which may already be the PasteWin panel).
func (m *Monitor) fallbackSourcePID(sequence uint32) uint32 {
	observed := m.foreground.Load()
	pid := uint32(observed)
	if uint32(observed>>32) != sequence || pid == windows.GetCurrentProcessId() {
		return 0
	}
	return pid
}

// Query only after releasing the clipboard; never extract an icon here.
func processPath(pid uint32) string { path, _ := processDetails(pid); return path }

func processDetails(pid uint32) (string, int64) {
	if pid == 0 {
		return "", 0
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		slog.Warn("source_process_open_failed", "pid", pid, "error", err)
		return "", 0
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, 32768)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		slog.Warn("source_process_path_failed", "pid", pid, "error", err)
		return "", 0
	}
	path := windows.UTF16ToString(buf[:size])
	// Avoid shell hosts (which misidentify packaged applications) and network I/O.
	if strings.HasPrefix(path, `\\`) || strings.HasSuffix(strings.ToLower(path), `\applicationframehost.exe`) {
		slog.Info("source_process_unsupported", "pid", pid, "source_path", path)
		return "", 0
	}
	var created, exit, kernel, user windows.Filetime
	if windows.GetProcessTimes(h, &created, &exit, &kernel, &user) != nil {
		return path, 0
	}
	return path, created.Nanoseconds()
}

type sourceProcess struct {
	path    string
	parent  uint32
	created int64
}

// ResolveSourceHost runs only on the capture worker, never in the clipboard
// listener or list renderer. Ordinary applications require no process snapshot.
func ResolveSourceHost(pid uint32, path string) string {
	if !strings.EqualFold(filepath.Base(path), "msedgewebview2.exe") {
		return path
	}
	handle, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)
	parents := make(map[uint32]uint32)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(handle, &entry); err == nil; err = windows.Process32Next(handle, &entry) {
		parents[entry.ProcessID] = entry.ParentProcessID
	}
	host := resolveWebViewHost(pid, func(id uint32) sourceProcess {
		path, created := processDetails(id)
		return sourceProcess{path, parents[id], created}
	})
	slog.Info("source_host_resolved", "pid", pid, "process_path", path, "source_path", host, "identified", host != "")
	return host
}
func resolveWebViewHost(pid uint32, lookup func(uint32) sourceProcess) string {
	current := lookup(pid)
	for range 8 {
		if !strings.EqualFold(filepath.Base(current.path), "msedgewebview2.exe") {
			return ""
		}
		if current.parent == 0 || current.parent == pid || current.created == 0 {
			return ""
		}
		parent := lookup(current.parent)
		if parent.path == "" || parent.created == 0 || parent.created > current.created {
			return ""
		}
		if !strings.EqualFold(filepath.Base(parent.path), "msedgewebview2.exe") {
			if strings.EqualFold(filepath.Base(parent.path), "explorer.exe") || strings.EqualFold(filepath.Base(parent.path), "msedge.exe") {
				return ""
			}
			return parent.path
		}
		pid = current.parent
		current = parent
	}
	return ""
}

// ApplicationIcon extracts a fixed-size PNG without invoking shell extensions.
// Call from a background worker, never from the clipboard listener.
func ApplicationIcon(path string) ([]byte, error) {
	if len(path) < 3 || path[1] != ':' || path[2] != '\\' {
		return nil, errors.New("unsupported icon path")
	}
	root, err := windows.UTF16PtrFromString(path[:3])
	if err != nil || windows.GetDriveType(root) != windows.DRIVE_FIXED {
		return nil, errors.New("icon must be on a local fixed drive")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var icon uintptr
	windows.NewLazySystemDLL("shell32.dll").NewProc("ExtractIconExW").Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&icon)), 0, 1)
	if icon == 0 {
		return nil, errors.New("no application icon")
	}
	defer user32.NewProc("DestroyIcon").Call(icon)
	gdi := windows.NewLazySystemDLL("gdi32.dll")
	dc, _, _ := gdi.NewProc("CreateCompatibleDC").Call(0)
	if dc == 0 {
		return nil, errors.New("icon DC failed")
	}
	defer gdi.NewProc("DeleteDC").Call(dc)
	// BITMAPINFOHEADER + one unused RGBQUAD, top-down 32-bit BGRA.
	type bitmapInfo struct {
		Size                   uint32
		Width, Height          int32
		Planes, Bits           uint16
		Compression, ImageSize uint32
		X, Y                   int32
		Used, Important, Color uint32
	}
	info := bitmapInfo{Size: 40, Width: 32, Height: -32, Planes: 1, Bits: 32}
	var bits uintptr
	bmp, _, _ := gdi.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == 0 {
		return nil, errors.New("icon bitmap failed")
	}
	defer gdi.NewProc("DeleteObject").Call(bmp)
	old, _, _ := gdi.NewProc("SelectObject").Call(dc, bmp)
	defer gdi.NewProc("SelectObject").Call(dc, old)
	pixels := make([]byte, 32*32*4)
	copyMemory.Call(bits, uintptr(unsafe.Pointer(&pixels[0])), uintptr(len(pixels)))
	ok, _, _ := user32.NewProc("DrawIconEx").Call(dc, 0, 0, icon, 32, 32, 0, 0, 3)
	if ok == 0 {
		return nil, errors.New("draw icon failed")
	}
	gdi.NewProc("GdiFlush").Call()
	copyMemory.Call(uintptr(unsafe.Pointer(&pixels[0])), bits, uintptr(len(pixels)))
	hasAlpha := false
	for i := 3; i < len(pixels); i += 4 {
		if pixels[i] != 0 {
			hasAlpha = true
			break
		}
	}
	if !hasAlpha {
		// Legacy icons carry transparency in a separate AND mask.
		if ok, _, _ := user32.NewProc("DrawIconEx").Call(dc, 0, 0, icon, 32, 32, 0, 0, 1); ok == 0 {
			return nil, errors.New("icon mask failed")
		}
		gdi.NewProc("GdiFlush").Call()
		mask := make([]byte, len(pixels))
		copyMemory.Call(uintptr(unsafe.Pointer(&mask[0])), bits, uintptr(len(mask)))
		for i := 0; i < len(pixels); i += 4 {
			pixels[i+3] = 255 - mask[i]
		}
	}
	for i := 0; i < len(pixels); i += 4 {
		pixels[i], pixels[i+2] = pixels[i+2], pixels[i]
	}
	img := &image.RGBA{Pix: pixels, Stride: 128, Rect: image.Rect(0, 0, 32, 32)}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
