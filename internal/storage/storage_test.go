package storage

import (
   "os"
   "path/filepath"
   "testing"
)

// TestConfigDirUsesXDG verifies ConfigDir uses XDG_CONFIG_HOME.
func TestConfigDirUsesXDG(t *testing.T) {
   tmp := t.TempDir()
   os.Setenv("XDG_CONFIG_HOME", tmp)
   defer os.Unsetenv("XDG_CONFIG_HOME")
   dir := ConfigDir()
   want := filepath.Join(tmp, "p2p-irc")
   if dir != want {
       t.Errorf("ConfigDir = %q; want %q", dir, want)
   }
}

// TestConfigDirUsesHome verifies ConfigDir falls back to HOME if XDG not set.
func TestConfigDirUsesHome(t *testing.T) {
   tmp := t.TempDir()
   os.Unsetenv("XDG_CONFIG_HOME")
   os.Setenv("HOME", tmp)
   defer os.Unsetenv("HOME")
   dir := ConfigDir()
   want := filepath.Join(tmp, ".config", "p2p-irc")
   if dir != want {
       t.Errorf("ConfigDir = %q; want %q", dir, want)
   }
}

// TestDataDirUsesXDG verifies DataDir uses XDG_DATA_HOME.
func TestDataDirUsesXDG(t *testing.T) {
   tmp := t.TempDir()
   os.Setenv("XDG_DATA_HOME", tmp)
   defer os.Unsetenv("XDG_DATA_HOME")
   dir := DataDir()
   want := filepath.Join(tmp, "p2p-irc")
   if dir != want {
       t.Errorf("DataDir = %q; want %q", dir, want)
   }
}

// TestDataDirUsesHome verifies DataDir falls back to HOME if XDG_DATA_HOME not set.
func TestDataDirUsesHome(t *testing.T) {
   tmp := t.TempDir()
   os.Unsetenv("XDG_DATA_HOME")
   os.Setenv("HOME", tmp)
   defer os.Unsetenv("HOME")
   dir := DataDir()
   want := filepath.Join(tmp, ".local", "share", "p2p-irc")
   if dir != want {
       t.Errorf("DataDir = %q; want %q", dir, want)
   }
}
