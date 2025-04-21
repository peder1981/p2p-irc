package main

import (
   "github.com/rivo/tview"

   "github.com/peder1981/p2p-irc/internal/ui"
)

func main() {
	ui := ui.NewUI()

	// Create a simple text view tab
	textView := tview.NewTextView().
		SetText("Welcome to p2p-irc-tui!\n\nThis is the main chat view.").
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true)
	ui.AddTab("Chat", textView)

	// Add a tab with instructions
	instructions := tview.NewTextView().
		SetText("Instructions:\n- Use Tab keys to switch tabs.\n- Press Ctrl+C to exit.").
		SetTextAlign(tview.AlignLeft)
	ui.AddTab("Help", instructions)

	if err := ui.Run(); err != nil {
		panic(err)
	}
}
