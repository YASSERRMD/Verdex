package encryption

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

// Encrypt authenticates and encrypts plaintext with source's current
// key, using AES-256-GCM with a fresh random nonce, and returns the
// serialized, versioned Envelope bytes. The returned ciphertext is
// safe to store: it carries the key ID needed to decrypt it later,
// even after source's current key has rotated (see KeySource's doc
// comment).
//
// Encrypt is the general-purpose entry point for any sensitive field
// value (a name, a national ID, free-text case content, ...). For
// wrapping an opaque backup blob, prefer EncryptBackup (backup.go) --
// same mechanism, a distinct name for call-site clarity.
func Encrypt(ctx context.Context, source KeySource, plaintext []byte) ([]byte, error) {
	if source == nil {
		return nil, ErrNilKeySource
	}
	if len(plaintext) == 0 {
		return nil, ErrEmptyPlaintext
	}

	key, err := source.CurrentKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("encryption: encrypt: resolve current key: %w", err)
	}
	if err := key.validate(); err != nil {
		return nil, fmt.Errorf("encryption: encrypt: %w", err)
	}

	gcm, err := newGCM(key.Material)
	if err != nil {
		return nil, fmt.Errorf("encryption: encrypt: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("encryption: encrypt: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	env := Envelope{
		Version:    EnvelopeV1,
		KeyID:      key.ID,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}
	encoded, err := encodeEnvelope(env)
	if err != nil {
		return nil, fmt.Errorf("encryption: encrypt: encode envelope: %w", err)
	}
	return encoded, nil
}

// Decrypt parses envelope (as produced by Encrypt), resolves the key
// it names via source (regardless of whether that key is still
// source's current key), and authenticates and decrypts it. Decrypt
// returns ErrAuthenticationFailed if the ciphertext was tampered with
// or was encrypted under a different key than the envelope claims.
func Decrypt(ctx context.Context, source KeySource, envelope []byte) ([]byte, error) {
	if source == nil {
		return nil, ErrNilKeySource
	}
	if len(envelope) == 0 {
		return nil, ErrEmptyCiphertext
	}

	env, err := decodeEnvelope(envelope)
	if err != nil {
		return nil, err
	}

	key, err := source.Key(ctx, env.KeyID)
	if err != nil {
		return nil, fmt.Errorf("encryption: decrypt: resolve key %q: %w", env.KeyID, err)
	}
	if err := key.validate(); err != nil {
		return nil, fmt.Errorf("encryption: decrypt: %w", err)
	}

	gcm, err := newGCM(key.Material)
	if err != nil {
		return nil, fmt.Errorf("encryption: decrypt: %w", err)
	}

	if len(env.Nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("%w: nonce size mismatch", ErrInvalidEnvelope)
	}

	plaintext, err := gcm.Open(nil, env.Nonce, env.Ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAuthenticationFailed, err)
	}
	return plaintext, nil
}

// newGCM constructs an AES-256-GCM AEAD cipher from a 32-byte key.
func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("construct AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("construct GCM: %w", err)
	}
	return gcm, nil
}
