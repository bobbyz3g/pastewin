package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

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
	s.wg.Add(1)
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

func (s *ClipboardService) capture() {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if s.paused {
		return
	}
	seq := native.Sequence()
	if seq == s.ignoreSequence {
		return
	}
	text, err := s.monitor.ReadText()
	if err != nil {
		s.report(err)
		return
	}
	if text == "" {
		return
	}
	if err = s.store.Add(text); err != nil {
		s.report(err)
		return
	}
	s.changed()
}
func (s *ClipboardService) changed()         { s.app.Event.Emit("history:changed") }
func (s *ClipboardService) report(err error) { s.app.Event.Emit("app:error", err.Error()) }

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
	if err := s.store.ClearUnpinned(); err != nil {
		return err
	}
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
	// Resuming must not capture data copied during a paused interval.
	s.ignoreSequence = native.Sequence()
	s.app.Event.Emit("status:changed")
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
	e, err := s.store.Get(id)
	if err != nil {
		return err
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	seq, err := s.monitor.WriteText(e.Text)
	if err == nil {
		s.ignoreSequence = seq
	}
	return err
}
func (s *ClipboardService) Paste(id int64) error {
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
	if err := s.Copy(id); err != nil {
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
