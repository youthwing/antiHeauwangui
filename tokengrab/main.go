package main

import (
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"wangui-tokengrab/internal/lock"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Single-instance: refuse to start a second copy while one is running.
	l, err := lock.Acquire()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		// We could show a Win32 MessageBox here; for a one-line refusal a
		// stderr line is fine — the user will see the empty window flash
		// and close, or run from a console.
		os.Exit(1)
	}
	defer l.Release()

	app := NewApp()

	err = wails.Run(&options.App{
		Title:         "wangui",
		Width:         560,
		Height:        780,
		MinWidth:      560,
		MinHeight:     780,
		MaxWidth:      560,
		MaxHeight:     780,
		DisableResize: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 9, G: 9, B: 11, A: 255}, // zinc-950
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind:       []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
