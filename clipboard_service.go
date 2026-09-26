package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"pastewin/internal/clipimage"
	"pastewin/internal/history"
	"pastewin/internal/native"
	"pastewin/internal/startup"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type Status struct {
	Paused       bool   `json:"paused"`
	Hotkey       string `json:"hotkey"`
	HotkeyError  string `json:"hotkeyError"`
	DatabasePath string `json:"databasePath"`
	CanPaste     bool   `json:"canPaste"`
}

type ClipboardService struct {
	app            *application.App
	window         *application.WebviewWindow
	store          *history.Store
	monitor        *native.Monitor
	cancel         context.CancelFunc
	ctx            context.Context
	wg             sync.WaitGroup
	opMu           sync.Mutex
	stateMu        sync.Mutex
	paused         bool
	ignoreSequence uint32
	target         uintptr
	hotkeyError    string
	databasePath   string
	pasting        bool
	captureEpoch   uint64
	lastCaptured   uint32
	captures       chan captureJob
}

func (s *ClipboardService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	dir := os.Getenv("PASTEWIN_DATA_DIR")
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		dir = filepath.Join(base, "PasteWin")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	s.databasePath = filepath.Join(dir, "history.db")
	store, err := history.Open(s.databasePath)
	if err != nil {
		return err
	}
	s.store = store
	s.paused = store.Paused()
	s.ctx, s.cancel = context.WithCancel(ctx)
	m, err := native.Watch(s.ctx)
	if err != nil {
		s.cancel()
		store.Close()
		return err
	}
	s.monitor = m
	s.captures = make(chan captureJob, 1)
	s.wg.Add(2)
	go s.processCaptures()
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-m.Changes:
				s.capture()
			}
		}
	}()
	return nil
}

func (s *ClipboardService) ServiceShutdown() error {
	s.cancel()
	s.monitor.Wait()
	s.wg.Wait()
	return s.store.Close()
}

func (s *ClipboardService) changed() {
	if s.app != nil {
		s.app.Event.Emit("history:changed")
	}
}
func (s *ClipboardService) report(err error) {
	if s.app != nil {
		s.app.Event.Emit("app:error", err.Error())
	}
}

func (s *ClipboardService) List(query string, pinned bool, offset int) (history.Page, error) {
	return s.store.List(query, pinned, offset)
}
func (s *ClipboardService) Get(id int64) (history.Entry, error) { return s.store.Get(id) }
func (s *ClipboardService) Pin(id int64, pinned bool) error {
	if err := s.store.Pin(id, pinned); err != nil {
		return err
	}
	s.changed()
	return nil
}
func (s *ClipboardService) Delete(id int64) error {
	if err := s.store.Delete(id); err != nil {
		return err
	}
	s.changed()
	return nil
}
func (s *ClipboardService) ClearUnpinned() error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.store.ClearUnpinned(); err != nil {
		return err
	}
	s.captureEpoch++
	s.changed()
	return nil
}

func (s *ClipboardService) SetPaused(paused bool) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.store.SetPaused(paused); err != nil {
		return err
	}
	s.paused = paused
	s.captureEpoch++
	// Resuming must not capture data copied during a paused interval.
	s.ignoreSequence = native.Sequence()
	if s.app != nil {
		s.app.Event.Emit("status:changed")
	}
	return nil
}

func (s *ClipboardService) AutoStart() (bool, error) { return startup.Enabled() }

func (s *ClipboardService) SetAutoStart(enabled bool) (bool, error) {
	if err := startup.SetEnabled(enabled); err != nil {
		return false, err
	}
	return startup.Enabled()
}

func (s *ClipboardService) Status() Status {
	s.opMu.Lock()
	paused := s.paused
	s.opMu.Unlock()
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return Status{paused, "Ctrl+Shift+V", s.hotkeyError, s.databasePath, native.ExternalWindow(s.target)}
}
func (s *ClipboardService) Copy(id int64) error {
	return s.copyContent(id, false)
}
func (s *ClipboardService) CopyImage(id int64) error {
	return s.copyContent(id, true)
}
func (s *ClipboardService) copyContent(id int64, imageOnly bool) error {
	e, err := s.store.Get(id)
	if err != nil {
		return err
	}
	useImage := imageOnly || (e.HasImage && strings.TrimSpace(e.Text) == "")
	var pngData, dib []byte
	if useImage {
		pngData, err = s.store.Image(id, false)
		if err != nil {
			return err
		}
		pixels, decodeErr := clipimage.DecodePNG(pngData)
		if decodeErr != nil {
			return decodeErr
		}
		dib, err = clipimage.EncodeDIBV5(pixels)
		if err != nil {
			return err
		}
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	var seq uint32
	if useImage {
		seq, err = s.monitor.WriteImage(pngData, dib)
	} else {
		seq, err = s.monitor.WriteText(e.Text)
	}
	if seq != 0 {
		s.ignoreSequence = seq
	}
	return err
}
func (s *ClipboardService) Paste(id int64) error {
	return s.pasteContent(id, false)
}
func (s *ClipboardService) PasteImage(id int64) error {
	return s.pasteContent(id, true)
}
func (s *ClipboardService) pasteContent(id int64, imageOnly bool) error {
	s.stateMu.Lock()
	if s.pasting {
		s.stateMu.Unlock()
		return errors.New("正在粘贴，请稍候")
	}
	target := s.target
	if !native.ExternalWindow(target) {
		s.stateMu.Unlock()
		return errors.New("请在目标应用中按 Ctrl+Shift+V 打开面板；也可以点击复制")
	}
	s.pasting = true
	s.stateMu.Unlock()
	defer func() { s.stateMu.Lock(); s.pasting = false; s.stateMu.Unlock() }()
	if err := s.copyContent(id, imageOnly); err != nil {
		return err
	}
	s.window.Hide()
	if err := native.Paste(s.ctx.Done(), target); err != nil {
		s.window.Show()
		s.window.Focus()
		return err
	}
	return nil
}
func (s *ClipboardService) Hide() { s.window.Hide() }
func (s *ClipboardService) Quit() { s.app.Quit() }

func (s *ClipboardService) show() {
	h := native.Foreground()
	s.stateMu.Lock()
	if native.ExternalWindow(h) {
		s.target = h
	}
	s.stateMu.Unlock()
	s.window.Show()
	s.window.Focus()
	s.app.Event.Emit("panel:shown")
}
