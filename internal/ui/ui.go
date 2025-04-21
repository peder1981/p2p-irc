package ui

import (
   "github.com/rivo/tview"
)

// UI represents the terminal user interface application.
type UI struct {
   app       *tview.Application
   pages     *tview.Pages
   firstPage bool
}

// NewUI creates a new UI instance.
func NewUI() *UI {
   return &UI{
       app:       tview.NewApplication(),
       pages:     tview.NewPages(),
       firstPage: true,
   }
}

// AddTab adds a new tab with the given name and content.
// The first added tab is shown initially; further tabs are hidden.
func (u *UI) AddTab(name string, content tview.Primitive) {
   show := u.firstPage
   u.pages.AddPage(name, content, true, show)
   if u.firstPage {
       u.app.SetRoot(u.pages, true)
       u.firstPage = false
   }
}

// Run starts the UI application.
func (u *UI) Run() error {
   return u.app.Run()
}