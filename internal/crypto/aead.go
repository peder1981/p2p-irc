package crypto

import (
    "crypto/rand"
    "golang.org/x/crypto/chacha20poly1305"
)

// Encrypt encrypts plaintext with the given secret key using ChaCha20-Poly1305.
// Returns a random nonce and the ciphertext.
func Encrypt(secretKey, plaintext, additionalData []byte) (nonce, ciphertext []byte, err error) {
    aead, err := chacha20poly1305.New(secretKey)
    if err != nil {
        return nil, nil, err
    }
    nonce = make([]byte, chacha20poly1305.NonceSize)
    if _, err = rand.Read(nonce); err != nil {
        return nil, nil, err
    }
    ciphertext = aead.Seal(nil, nonce, plaintext, additionalData)
    return nonce, ciphertext, nil
}

// Decrypt decrypts ciphertext with the given secret key and nonce using ChaCha20-Poly1305.
// Returns the plaintext or an error if decryption fails.
func Decrypt(secretKey, nonce, ciphertext, additionalData []byte) ([]byte, error) {
    aead, err := chacha20poly1305.New(secretKey)
    if err != nil {
        return nil, err
    }
    plaintext, err := aead.Open(nil, nonce, ciphertext, additionalData)
    if err != nil {
        return nil, err
    }
    return plaintext, nil
}