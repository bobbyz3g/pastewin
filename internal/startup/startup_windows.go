// Package startup manages the current user's PasteWin logon registration.
package startup

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows/registry"
)

const Argument = "--autostart"
const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const valueName = "dev.pastewin.app"

type registration struct {
	keyPath    string
	executable string
}

func current() (registration, error) {
	executable, err := os.Executable()
	if err != nil {
		return registration{}, fmt.Errorf("无法获取程序路径：%w", err)
	}
	return registration{runKey, executable}, nil
}

// Enabled reports whether the Run entry points to this executable.
// Windows startup-app settings and administrative policy can override it.
func Enabled() (bool, error) {
	r, err := current()
	if err != nil {
		return false, err
	}
	return r.enabled()
}

func SetEnabled(enabled bool) error {
	r, err := current()
	if err != nil {
		return err
	}
	return r.setEnabled(enabled)
}

func (r registration) command() string {
	return `"` + r.executable + `" ` + Argument
}

func (r registration) enabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, r.keyPath, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("无法读取开机自启设置：%w", err)
	}
	defer key.Close()
	command, _, err := key.GetStringValue(valueName)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("无法读取开机自启设置：%w", err)
	}
	return strings.EqualFold(command, r.command()), nil
}

func (r registration) setEnabled(enabled bool) error {
	if !enabled {
		key, err := registry.OpenKey(registry.CURRENT_USER, r.keyPath, registry.SET_VALUE)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("无法关闭开机自启：%w", err)
		}
		defer key.Close()
		err = key.DeleteValue(valueName)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("无法关闭开机自启：%w", err)
		}
		return nil
	}
	// Run commands are limited to 260 UTF-16 code units, including arguments.
	if len(utf16.Encode([]rune(r.command()))) > 260 {
		return errors.New("程序路径过长，无法设置开机自启；请将 PasteWin 移到较短的路径")
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, r.keyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("无法开启开机自启：%w", err)
	}
	defer key.Close()
	if err = key.SetStringValue(valueName, r.command()); err != nil {
		return fmt.Errorf("无法开启开机自启：%w", err)
	}
	return nil
}
