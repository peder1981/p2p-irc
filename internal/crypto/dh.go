package crypto

import (
    "crypto/ecdh"
    "crypto/rand"
)

// X25519KeyPair holds a private and public key for X25519 key agreement.
type X25519KeyPair struct {
    PrivateKey []byte
    PublicKey  []byte
}

// GenerateX25519KeyPair generates a new X25519 key pair.
func GenerateX25519KeyPair() (*X25519KeyPair, error) {
    curve := ecdh.X25519()
    priv, err := curve.GenerateKey(rand.Reader)
    if err != nil {
        return nil, err
    }
    pub := priv.PublicKey()
    return &X25519KeyPair{
        PrivateKey: priv.Bytes(),
        PublicKey:  pub.Bytes(),
    }, nil
}

// DeriveSharedSecret derives a shared secret using own private key and peer's public key.
func DeriveSharedSecret(ownPrivateKey, peerPublicKey []byte) ([]byte, error) {
    curve := ecdh.X25519()
    priv, err := curve.NewPrivateKey(ownPrivateKey)
    if err != nil {
        return nil, err
    }
    peerPub, err := curve.NewPublicKey(peerPublicKey)
    if err != nil {
        return nil, err
    }
    secret, err := priv.ECDH(peerPub)
    if err != nil {
        return nil, err
    }
    return secret, nil
}