package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

func main() {
	app := mygo.App
	app.SetName("Looma")

	dataDir, err := os.UserConfigDir()
	if err != nil {
		dataDir = os.TempDir()
	}
	svc, err := NewAppService(filepath.Join(dataDir, "looma"))
	if err != nil {
		log.Fatal(err)
	}
	mygo.Bind(svc)

	var mainWin *mygo.Window
	openMain := func() {
		if mainWin != nil {
			mainWin.Show()
			return
		}
		mainWin = mygo.NewWindow(mygo.WindowOptions{
			Title:     "Looma",
			Width:     1040,
			Height:    720,
			MinWidth:  380,
			MinHeight: 560,
		})
	}

	app.WhenReady(func() {
		openMain()
	})

	// Keep alive in SNI tray on Linux (niri/Wayland) when window closes
	app.OnWindowAllClosed(func() {
		mainWin = nil
	})
	app.OnActivate(func(hasVisibleWindows bool) {
		if !hasVisibleWindows {
			openMain()
		}
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
