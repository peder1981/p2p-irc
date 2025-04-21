package crypto

import (
    "bytes"
    "crypto/ed25519"
    "crypto/rand"
    "os"
    "path/filepath"
    "testing"
)

func TestLoadOrCreateIdentity(t *testing.T) {
    // Use a temp config directory
    tmp := t.TempDir()
    os.Setenv("XDG_CONFIG_HOME", tmp)
    // First call should create a new key
    priv1, err := LoadOrCreateIdentity()
    if err != nil {
        t.Fatalf("LoadOrCreateIdentity first call error: %v", err)
    }
    // Second call should load the same key
    priv2, err := LoadOrCreateIdentity()
    if err != nil {
        t.Fatalf("LoadOrCreateIdentity second call error: %v", err)
    }
    if !bytes.Equal(priv1, priv2) {
        t.Errorf("Keys do not match: %x vs %x", priv1, priv2)
    }
    // Identity file should exist
    keyPath := filepath.Join(tmp, "p2p-irc", "identity.pem")
    if _, err := os.Stat(keyPath); err != nil {
        t.Errorf("identity.pem file not found at %s: %v", keyPath, err)
    }
}

func TestSignVerify(t *testing.T) {
    pub, priv, err := ed25519.GenerateKey(rand.Reader)
    if err != nil {
        t.Fatalf("GenerateKey error: %v", err)
    }
    msg := []byte("hello world")
    sig := Sign(priv, msg)
    if !Verify(pub, msg, sig) {
        t.Errorf("Verify failed for valid signature")
    }
    // Tamper with signature
    badSig := make([]byte, len(sig))
    copy(badSig, sig)
    badSig[0] ^= 0xFF
    if Verify(pub, msg, badSig) {
        t.Errorf("Verify succeeded for invalid signature")
    }
}

func TestX25519KeyExchange(t *testing.T) {
    kp1, err := GenerateX25519KeyPair()
    if err != nil {
        t.Fatalf("GenerateX25519KeyPair 1 error: %v", err)
    }
    kp2, err := GenerateX25519KeyPair()
    if err != nil {
        t.Fatalf("GenerateX25519KeyPair 2 error: %v", err)
    }
    secret1, err := DeriveSharedSecret(kp1.PrivateKey, kp2.PublicKey)
    if err != nil {
        t.Fatalf("DeriveSharedSecret 1 error: %v", err)
    }
    secret2, err := DeriveSharedSecret(kp2.PrivateKey, kp1.PublicKey)
    if err != nil {
        t.Fatalf("DeriveSharedSecret 2 error: %v", err)
    }
    if !bytes.Equal(secret1, secret2) {
        t.Errorf("Shared secrets do not match")
    }
}

func TestEncryptDecrypt(t *testing.T) {
    // Derive a shared secret
    kp1, err := GenerateX25519KeyPair()
    if err != nil {
        t.Fatalf("GenerateX25519KeyPair error: %v", err)
    }
    kp2, err := GenerateX25519KeyPair()
    if err != nil {
        t.Fatalf("GenerateX25519KeyPair error: %v", err)
    }
    secret, err := DeriveSharedSecret(kp1.PrivateKey, kp2.PublicKey)
    if err != nil {
        t.Fatalf("DeriveSharedSecret error: %v", err)
    }
    plaintext := []byte("secret message")
    aad := []byte("aad")
    nonce, ciphertext, err := Encrypt(secret, plaintext, aad)
    if err != nil {
        t.Fatalf("Encrypt error: %v", err)
    }
    decrypted, err := Decrypt(secret, nonce, ciphertext, aad)
    if err != nil {
        t.Fatalf("Decrypt error: %v", err)
    }
    if !bytes.Equal(decrypted, plaintext) {
        t.Errorf("Decrypted message mismatch: got %s, want %s", decrypted, plaintext)
    }
    // Tampering detection
    ciphertext[0] ^= 0xFF
    if _, err := Decrypt(secret, nonce, ciphertext, aad); err == nil {
        t.Errorf("Decrypt succeeded on tampered ciphertext")
    }
}