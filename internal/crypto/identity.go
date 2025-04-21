package crypto

import (
    "crypto/ed25519"
    "crypto/rand"
    "encoding/pem"
    "io/ioutil"
    "os"
    "path/filepath"

    "github.com/peder1981/p2p-irc/internal/storage"
)

// LoadOrCreateIdentity carrega a chave ed25519 de id ou cria uma nova, armazenando em config dir com permissão 0600.
func LoadOrCreateIdentity() (ed25519.PrivateKey, error) {
    cfgDir := storage.ConfigDir()
    os.MkdirAll(cfgDir, 0700)
    keyPath := filepath.Join(cfgDir, "identity.pem")
    if data, err := ioutil.ReadFile(keyPath); err == nil {
        block, _ := pem.Decode(data)
        if block != nil {
            return ed25519.PrivateKey(block.Bytes), nil
        }
    }
    _, priv, err := ed25519.GenerateKey(rand.Reader)
    if err != nil {
        return nil, err
    }
    block := &pem.Block{Type: "ED25519 PRIVATE KEY", Bytes: priv}
    pemData := pem.EncodeToMemory(block)
    if err := ioutil.WriteFile(keyPath, pemData, 0600); err != nil {
        return nil, err
    }
    return priv, nil
}
