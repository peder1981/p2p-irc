package storage

import (
    "os"
    "path/filepath"
)

// ConfigDir retorna o diretório de configuração seguindo XDG_CONFIG_HOME ou ~/.config
func ConfigDir() string {
    if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
        return filepath.Join(xdg, "p2p-irc")
    }
    if home := os.Getenv("HOME"); home != "" {
        return filepath.Join(home, ".config", "p2p-irc")
    }
    return ""
}

// DataDir retorna o diretório de dados seguindo XDG_DATA_HOME ou ~/.local/share
func DataDir() string {
    if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
        return filepath.Join(xdg, "p2p-irc")
    }
    if home := os.Getenv("HOME"); home != "" {
        return filepath.Join(home, ".local", "share", "p2p-irc")
    }
    return ""
}
