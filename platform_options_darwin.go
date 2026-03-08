//go:build darwin

package main

import (
	"github.com/wailsapp/wails/v2/pkg/options"
	macopts "github.com/wailsapp/wails/v2/pkg/options/mac"
)

func applyPlatformOptions(appOptions *options.App) {
	appOptions.Mac = &macopts.Options{
		TitleBar: &macopts.TitleBar{
			TitlebarAppearsTransparent: true,
			HideTitle:                  false,
			HideTitleBar:               false,
			FullSizeContent:            false,
			UseToolbar:                 false,
			HideToolbarSeparator:       true,
		},
		Appearance:           macopts.DefaultAppearance,
		WebviewIsTransparent: false,
		WindowIsTranslucent:  false,
		About: &macopts.AboutInfo{
			Title:   appTitle,
			Message: "Desktop manager for CPA Codex auth pools.",
		},
	}
}
