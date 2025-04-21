package ui

import (
   "testing"
   "github.com/rivo/tview"
)

// TestNewUI ensures NewUI returns a non-nil UI instance.
func TestNewUI(t *testing.T) {
   u := NewUI()
   if u == nil {
       t.Error("NewUI returned nil")
   }
}

// TestAddTabNoPanic ensures AddTab does not panic when adding a tab.
func TestAddTabNoPanic(t *testing.T) {
   u := NewUI()
   box := tview.NewBox()
   u.AddTab("Test", box)
}
