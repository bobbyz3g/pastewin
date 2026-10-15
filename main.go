package main

import (
	"bytes"
	"embed"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"slices"

	"pastewin/internal/startup"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
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
		Frameless: true, BackgroundColour: application.NewRGB(234, 233, 241), URL: "/",
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
			service.hotkeyError = "快捷键已被占用，请从托盘打开面板"
			service.stateMu.Unlock()
			app.Event.Emit("status:changed")
		}
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
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
