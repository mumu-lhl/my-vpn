package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/egoist/mygo"
)

func main() {
	app := mygo.App
	app.SetName("Looma")

	// Keep a single desktop instance so re-launching while resident in the SNI
	// tray focuses/restores the existing window instead of spawning a duplicate.
	if runtime.GOOS != "android" && runtime.GOOS != "ios" {
		if !app.RequestSingleInstanceLock() {
			return
		}
	}

	dataDir, err := os.UserConfigDir()
	if err != nil {
		dataDir = os.TempDir()
	}
	svc, err := NewAppService(filepath.Join(dataDir, "looma"))
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(io.MultiWriter(os.Stderr, svc.LogWriter()))
	mygo.Bind(svc)

	var (
		winMu    sync.Mutex
		mainWin  *mygo.Window
		trayCtrl *TrayController
		quitOnce sync.Once
	)

	openMain := func() {
		winMu.Lock()
		win := mainWin
		if win != nil && !win.IsDestroyed() {
			winMu.Unlock()
			win.Show()
			win.Focus()
			return
		}
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "Looma",
			URL:       "/",
			Width:     1040,
			Height:    720,
			MinWidth:  380,
			MinHeight: 560,
		})
		mainWin = win
		winMu.Unlock()
	}

	cleanup := func() {
		quitOnce.Do(func() {
			if trayCtrl != nil {
				trayCtrl.Destroy()
			}
			if err := svc.Shutdown(); err != nil {
				log.Printf("looma: shutdown error: %v", err)
			}
		})
	}

	quitApp := func() {
		cleanup()
		app.Quit()
	}

	app.WhenReady(func() {
		if runtime.GOOS != "android" && runtime.GOOS != "ios" {
			tc, err := NewTrayController(svc, openMain, quitApp)
			if err != nil {
				log.Printf("looma: SNI tray unavailable (install libayatana-appindicator3 for tray residency): %v", err)
			} else {
				trayCtrl = tc
				svc.setOnStateChange(tc.Sync)
			}
		}
		openMain()
	})

	app.OnSecondInstance(func(_ []string, _ string) {
		openMain()
	})

	// When MinimizeToTray is enabled and the SNI tray is active, closing the
	// main window releases the WebView while keeping the Go backend and active
	// proxy tunnel running in the background (SPEC 5.1 / AC-11).
	app.OnWindowAllClosed(func() {
		winMu.Lock()
		mainWin = nil
		winMu.Unlock()

		if !ShouldKeepAliveInTray(svc.GetConfig(), trayCtrl.Active(), svc.GetStats().Running) {
			quitApp()
		}
	})

	app.OnActivate(func(hasVisibleWindows bool) {
		if !hasVisibleWindows {
			openMain()
		}
	})

	app.OnQuit(func() {
		cleanup()
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
