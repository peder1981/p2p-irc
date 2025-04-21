package crypto

import "crypto/ed25519"

// Sign returns a signature of the message using the Ed25519 private key.
func Sign(privateKey ed25519.PrivateKey, message []byte) []byte {
    return ed25519.Sign(privateKey, message)
}

// Verify returns true if the signature is a valid Ed25519 signature for the message and public key.
func Verify(publicKey ed25519.PublicKey, message, signature []byte) bool {
    return ed25519.Verify(publicKey, message, signature)
}

// PublicKeyFromPrivate derives the Ed25519 public key corresponding to the given private key.
func PublicKeyFromPrivate(privateKey ed25519.PrivateKey) ed25519.PublicKey {
    return privateKey.Public().(ed25519.PublicKey)
}