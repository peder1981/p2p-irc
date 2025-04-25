package scripts

import (
   "os"
   "path/filepath"
   "strings"

   "github.com/BurntSushi/toml"
)

// Config holds alias definitions.
type Config struct {
   Aliases map[string]string `toml:"alias"`
}

// Engine manages alias expansion and script loading.
type Engine struct {
   path string
   cfg  Config
}

// DefaultScriptsPath returns the default path for scripts.toml under XDG config.
func DefaultScriptsPath() (string, error) {
   dir, err := os.UserConfigDir()
   if err != nil {
       home, err2 := os.UserHomeDir()
       if err2 != nil {
           return "", err
       }
       dir = filepath.Join(home, ".config")
   }
   return filepath.Join(dir, "p2p-irc", "scripts.toml"), nil
}

// NewEngine creates a new Engine loading the scripts configuration.
func NewEngine(path string) (*Engine, error) {
   e := &Engine{
       path: path,
       cfg:  Config{Aliases: make(map[string]string)},
   }
   if err := e.Load(); err != nil {
       return nil, err
   }
   return e, nil
}

// Load reads the scripts file and decodes alias definitions.
func (e *Engine) Load() error {
   info, err := os.Stat(e.path)
   if err != nil {
       if os.IsNotExist(err) {
           e.cfg.Aliases = make(map[string]string)
           return nil
       }
       return err
   }
   if info.IsDir() {
       e.cfg.Aliases = make(map[string]string)
       return nil
   }
   var cfg Config
   if _, err := toml.DecodeFile(e.path, &cfg); err != nil {
       return err
   }
   if cfg.Aliases == nil {
       cfg.Aliases = make(map[string]string)
   }
   e.cfg = cfg
   return nil
}

// Reload reloads the scripts configuration from file.
func (e *Engine) Reload() error {
   return e.Load()
}

// Expand applies alias expansion to the given input line.
func (e *Engine) Expand(input string) string {
   parts := strings.Fields(input)
   if len(parts) == 0 {
       return input
   }
   name := parts[0]
   if exp, ok := e.cfg.Aliases[name]; ok {
       if len(parts) > 1 {
           return exp + " " + strings.Join(parts[1:], " ")
       }
       return exp
   }
   return input
}

// ListAliases returns the current alias map.
func (e *Engine) ListAliases() map[string]string {
   return e.cfg.Aliases
}

// AddAlias adds or updates an alias and saves to file.
func (e *Engine) AddAlias(name, expansion string) error {
   e.cfg.Aliases[name] = expansion
   return e.save()
}

// RemoveAlias deletes an alias and saves to file.
func (e *Engine) RemoveAlias(name string) error {
   delete(e.cfg.Aliases, name)
   return e.save()
}

// save writes the current Config to the scripts file.
func (e *Engine) save() error {
   dir := filepath.Dir(e.path)
   if err := os.MkdirAll(dir, 0o755); err != nil {
       return err
   }
   f, err := os.Create(e.path)
   if err != nil {
       return err
   }
   defer f.Close()
   enc := toml.NewEncoder(f)
   return enc.Encode(e.cfg)
}