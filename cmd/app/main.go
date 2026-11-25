package main

import (
	"log"

	"github.com/egoist/mygo"
)

func main() {
	app := mygo.App
	app.SetName("MyVPN")

	app.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:     "MyVPN",
			Width:     1040,
			Height:    720,
			MinWidth:  380,
			MinHeight: 560,
		})
	})

	app.OnWindowAllClosed(func() {})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
