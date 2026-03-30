// agocli - AuraGo CLI Tool
// Three modes: chat TUI (default), --setup wizard, --update wizard
package main

import (
	"flag"
	"fmt"
	"os"

	"aurago/cmd/agocli/chat"
	"aurago/cmd/agocli/shared"
	"aurago/cmd/agocli/setup"
	"aurago/cmd/agocli/update"

	"github.com/charmbracelet/bubbletea"
)

func main() {
	setupMode := flag.Bool("setup", false, "Run setup wizard")
	updateMode := flag.Bool("update", false, "Run update wizard")
	serverURL := flag.String("server", shared.GetServerURL(), "AuraGo server URL")
	flag.Usage = func() {
		fmt.Println("agocli - AuraGo CLI Tool")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  agocli            Start interactive chat TUI")
		fmt.Println("  agocli --setup    Run setup wizard")
		fmt.Println("  agocli --update   Run update wizard")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *setupMode {
		p := tea.NewProgram(setup.NewModel(*serverURL),
			tea.WithAltScreen(),
		)
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running setup: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *updateMode {
		p := tea.NewProgram(update.NewModel(*serverURL),
			tea.WithAltScreen(),
		)
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running update: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Default: chat TUI
	p := tea.NewProgram(chat.NewModel(*serverURL),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running chat: %v\n", err)
		os.Exit(1)
	}
}
