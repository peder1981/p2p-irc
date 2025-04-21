package config

import (
   "os"
   "path/filepath"

   "github.com/BurntSushi/toml"
)

// Config holds the application configuration loaded from TOML.
type Config struct {
   Network NetworkConfig `toml:"network"`
   DHT     DHTConfig     `toml:"dht"`
   Crypto  CryptoConfig  `toml:"crypto"`
   UI      UIConfig      `toml:"ui"`
}

// NetworkConfig holds network-related settings.
type NetworkConfig struct {
   BootstrapPeers []string `toml:"bootstrapPeers"`
   StunServers    []string `toml:"stunServers"`
}

// DHTConfig holds settings for the DHT datastore.
type DHTConfig struct {
   DatastorePath string `toml:"datastorePath"`
   TTLSeconds    int    `toml:"ttlSeconds"`
}

// CryptoConfig holds cryptography-related settings.
type CryptoConfig struct {
   // KeyDir is the directory where identity keys are stored.
   KeyDir string `toml:"keyDir"`
}

// UIConfig holds user-interface related settings.
type UIConfig struct {
   // EnableTabs toggles the TUI tab bar.
   EnableTabs bool `toml:"enableTabs"`
}

// NewDefaultConfig returns a Config populated with default values.
func NewDefaultConfig() Config {
   return Config{
       Network: NetworkConfig{
           BootstrapPeers: []string{},
           StunServers:    []string{"stun.l.google.com:19302"},
       },
       DHT: DHTConfig{
           DatastorePath: "",
           TTLSeconds:    60,
       },
       Crypto: CryptoConfig{
           KeyDir: "",
       },
       UI: UIConfig{
           EnableTabs: true,
       },
   }
}

// DefaultConfigPath returns the XDG default path for the config file.
func DefaultConfigPath() (string, error) {
   dir, err := os.UserConfigDir()
   if err != nil {
       home, err2 := os.UserHomeDir()
       if err2 != nil {
           return "", err
       }
       dir = filepath.Join(home, ".config")
   }
   return filepath.Join(dir, "p2p-irc", "config.toml"), nil
}

// Load reads the configuration from the given path (TOML).
// If path is empty, it uses the XDG default. Missing file returns defaults.
func Load(path string) (*Config, error) {
   cfg := NewDefaultConfig()
   // Determine config path
   if path == "" {
       defaultPath, err := DefaultConfigPath()
       if err != nil {
           return nil, err
       }
       path = defaultPath
   }
   // If file does not exist, return defaults
   if info, err := os.Stat(path); err != nil {
       if os.IsNotExist(err) {
           return &cfg, nil
       }
       return nil, err
   } else if info.IsDir() {
       return &cfg, nil
   }
   // Decode TOML into cfg
   if _, err := toml.DecodeFile(path, &cfg); err != nil {
       return nil, err
   }
   return &cfg, nil
}