// Command navmux は tmux / zellij をメニュー駆動で操作する TUI。
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jss826/navmux/internal/app"
	"github.com/jss826/navmux/internal/backend"
	"github.com/jss826/navmux/internal/env"
	"github.com/jss826/navmux/internal/ui"
	"github.com/jss826/navmux/internal/upgrade"
)

func main() {
	showVersion := flag.Bool("version", false, "バージョンを表示して終了する")
	flag.Parse()
	if *showVersion {
		bi, ok := debug.ReadBuildInfo()
		fmt.Println(app.FormatVersion(bi, ok))
		return
	}

	if flag.Arg(0) == "upgrade" {
		bi, ok := debug.ReadBuildInfo()
		if err := upgrade.NewRunner(app.Version(bi, ok)).Run(); err != nil {
			fmt.Fprintln(os.Stderr, "navmux upgrade:", err)
			os.Exit(1)
		}
		return
	}

	current := env.CurrentMux(env.OSLookup)

	all := []backend.Backend{backend.NewTmux(), backend.NewZellij()}
	bs := app.OrderedBackends(all, current)
	if len(bs) == 0 {
		fmt.Fprintln(os.Stderr, "navmux: tmux も zellij も見つかりません。どちらかをインストールしてください。")
		os.Exit(1)
	}

	p := tea.NewProgram(ui.New(bs, current))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "navmux:", err)
		os.Exit(1)
	}
}
