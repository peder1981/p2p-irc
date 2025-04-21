package config

import (
   "os"
   "path/filepath"
   "testing"
)

const sampleToml = `
  [network]
  bootstrapPeers = ["peer1:1000", "peer2:1000"]
  stunServers = ["stun.example:3478"]

  [dht]
  datastorePath = "/tmp/datastore"
  ttlSeconds = 120

  [crypto]
  keyDir = "/home/user/.config/p2p-irc"

  [ui]
  enableTabs = false
`

// TestLoadFromFile ensures Load reads values from a TOML file.
func TestLoadFromFile(t *testing.T) {
   tmp := t.TempDir()
   cfgPath := filepath.Join(tmp, "config.toml")
   if err := os.WriteFile(cfgPath, []byte(sampleToml), 0644); err != nil {
       t.Fatalf("failed to write sample config: %v", err)
   }
   cfg, err := Load(cfgPath)
   if err != nil {
       t.Fatalf("Load returned error: %v", err)
   }
   if len(cfg.Network.BootstrapPeers) != 2 || cfg.Network.BootstrapPeers[0] != "peer1:1000" {
       t.Errorf("unexpected bootstrap peers: %v", cfg.Network.BootstrapPeers)
   }
   if len(cfg.Network.StunServers) != 1 || cfg.Network.StunServers[0] != "stun.example:3478" {
       t.Errorf("unexpected stun servers: %v", cfg.Network.StunServers)
   }
   if cfg.DHT.DatastorePath != "/tmp/datastore" {
       t.Errorf("unexpected datastore path: %s", cfg.DHT.DatastorePath)
   }
   if cfg.DHT.TTLSeconds != 120 {
       t.Errorf("unexpected TTL: %d", cfg.DHT.TTLSeconds)
   }
   if cfg.Crypto.KeyDir != "/home/user/.config/p2p-irc" {
       t.Errorf("unexpected keyDir: %s", cfg.Crypto.KeyDir)
   }
   if cfg.UI.EnableTabs != false {
       t.Errorf("unexpected enableTabs: %v", cfg.UI.EnableTabs)
   }
}

// TestLoadDefaults ensures Load returns default values when file missing.
func TestLoadDefaults(t *testing.T) {
   cfg, err := Load("/path/does/not/exist/config.toml")
   if err != nil {
       t.Fatalf("Load returned error: %v", err)
   }
   def := NewDefaultConfig()
   if len(cfg.Network.StunServers) != len(def.Network.StunServers) || cfg.Network.StunServers[0] != def.Network.StunServers[0] {
       t.Errorf("defaults not applied: %v", cfg.Network.StunServers)
   }
   if cfg.DHT.TTLSeconds != def.DHT.TTLSeconds {
       t.Errorf("defaults not applied for TTL: %d", cfg.DHT.TTLSeconds)
   }
   if cfg.UI.EnableTabs != def.UI.EnableTabs {
       t.Errorf("defaults not applied for UI: %v", cfg.UI.EnableTabs)
   }
}