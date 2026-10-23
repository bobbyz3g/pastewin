package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"

	"pastewin/internal/diagnostics"
	"pastewin/internal/startup"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/info.json
var buildMetadata []byte

func main() { os.Exit(run()) }

func run() int {
	dir, err := dataDirectory()
	if err == nil {
		logs, openErr := diagnostics.Open(filepath.Join(dir, "logs"))
		if openErr == nil {
			slog.SetDefault(logs.Logger())
			defer logs.Close()
		} else {
			slog.Error("log_init_failed", "error", openErr)
		}
	} else {
		slog.Error("data_directory_failed", "error", err)
	}
	var version struct {
		Fixed struct {
			Version string `json:"file_version"`
		} `json:"fixed"`
	}
	_ = json.Unmarshal(buildMetadata, &version)
	revision := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				revision = setting.Value
			}
		}
	}
	slog.Info("app_start", "version", version.Fixed.Version, "revision", revision, "go", runtime.Version(), "os", runtime.GOOS, "arch", runtime.GOARCH, "pid", os.Getpid())
	defer slog.Info("app_stop")
	service := &ClipboardService{}
	app := application.New(application.Options{
		Name: "PasteWin", Description: "本地剪贴板工作台", Icon: trayIcon(),
		Services: []application.Service{application.NewService(service)},
		Assets:   application.AssetOptions{Handler: application.AssetFileServerFS(assets), Middleware: service.imageAssets},
		SingleInstance: &application.SingleInstanceOptions{UniqueID: "dev.pastewin.app", OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
			if service.window != nil && !slices.Contains(data.Args, startup.Argument) {
				service.show()
			}
		}},
	})
	service.app = app
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "PasteWin", Name: "main", Width: 360, Height: 680, MinWidth: 320, MinHeight: 420,
		Frameless: true, BackgroundColour: application.NewRGB(235, 240, 249), URL: "/",
		BackgroundType: application.BackgroundTypeTranslucent,
		Windows: application.WindowsWindow{
			BackdropType: application.Acrylic,
			Theme:        application.Light,
		},
		Hidden: slices.Contains(os.Args[1:], startup.Argument),
	})
	service.window = window
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) { window.Hide(); e.Cancel() })
	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon())
	tray.SetTooltip("PasteWin · Ctrl+Shift+V")
	menu := app.NewMenu()
	openItem := menu.Add("打开 PasteWin").OnClick(func(_ *application.Context) { service.show() })
	pauseItem := menu.Add("暂停 / 恢复记录").OnClick(func(_ *application.Context) {
		if err := service.SetPaused(!service.Status().Paused); err != nil {
			service.report(err)
		}
	})
	menu.AddSeparator()
	quitItem := menu.Add("退出").OnClick(func(_ *application.Context) { app.Quit() })
	tray.SetMenu(menu)
	service.updateTrayLanguage = func(language string) {
		application.InvokeSync(func() {
			if language == "en" {
				openItem.SetLabel("Open PasteWin")
				pauseItem.SetLabel("Pause / Resume capture")
				quitItem.SetLabel("Exit")
			} else {
				openItem.SetLabel("打开 PasteWin")
				pauseItem.SetLabel("暂停 / 恢复记录")
				quitItem.SetLabel("退出")
			}

		})
	}
	tray.OnClick(service.show)
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		if language, err := service.Language(); err == nil {
			service.updateTrayLanguage(language)
		}
		if err := app.GlobalShortcut.Register("Ctrl+Shift+V", service.show); err != nil {
			service.stateMu.Lock()
			slog.Warn("hotkey_register_failed", "error", err)
			service.hotkeyError = "快捷键已被占用，请从托盘打开面板"
			service.stateMu.Unlock()
			app.Event.Emit("status:changed")
		}
	})
	if err := app.Run(); err != nil {
		slog.Error("app_run_failed", "error", err)
		return 1
	}
	return 0
}

func trayIcon() []byte {
	i := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	blue := color.NRGBA{R: 47, G: 92, B: 224, A: 255}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for y := 2; y < 30; y++ {
		for x := 2; x < 30; x++ {
			i.Set(x, y, blue)
		}
	}
	for y := 9; y < 25; y++ {
		for x := 10; x < 23; x++ {
			if x == 10 || x == 22 || y == 9 || y == 24 {
				i.Set(x, y, white)
			}
		}
	}
	for y := 6; y < 11; y++ {
		for x := 13; x < 20; x++ {
			i.Set(x, y, white)
		}
	}
	var out bytes.Buffer
	_ = png.Encode(&out, i)
	return out.Bytes()
}
