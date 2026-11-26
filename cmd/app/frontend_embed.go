//go:build looma_embed_frontend

package main

import (
	"github.com/egoist/mygo"

	"github.com/mumu-lhl/my-vpn/frontend"
)

func init() {
	mygo.SetFrontend(frontend.Assets())
}
