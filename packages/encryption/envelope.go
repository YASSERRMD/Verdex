package encryption

import (
	"encoding/binary"
	"fmt"
)

// envelopeMagic identifies bytes produced by this package's envelope
// format, distinguishing them from arbitrary opaque data.
const envelopeMagic = "VDXE"

// EnvelopeV1 is the only envelope format version this phase defines.
// It is encoded as:
//
//	magic (4 bytes) | version (1 byte) | keyID length (2 bytes, BE) |
//	keyID (variable) | nonce length (1 byte) | nonce (variable) |
//	ciphertext (remainder, includes the GCM authentication tag)
//
// Recording the key ID in the envelope (in cleartext -- it is an
// identifier, not secret material) is what makes key rotation
// non-breaking: Decrypt reads the ID back out and asks the KeySource
// for that specific key, regardless of which key is current at
// decrypt time.
const EnvelopeV1 byte = 1

// Envelope is the parsed, versioned wrapper around a field-level or
// backup ciphertext. Callers normally do not construct or inspect an
// Envelope directly -- Encrypt/Decrypt and EncryptBackup/DecryptBackup
// handle encoding and decoding -- but it is exported so a caller that
// needs to inspect which key ID a stored ciphertext used (e.g. for an
// audit report) can do so without decrypting.
type Envelope struct {
	// Version identifies the envelope wire format. Only EnvelopeV1
	// exists today.
	Version byte

	// KeyID identifies which KeySource key encrypted Ciphertext.
	KeyID string

	// Nonce is the random GCM nonce used for this specific encryption.
	// Never reused across calls.
	Nonce []byte

	// Ciphertext is the AES-256-GCM output (ciphertext with the
	// authentication tag appended, as crypto/cipher.AEAD.Seal
	// produces).
	Ciphertext []byte
}

// encodeEnvelope serializes env into the wire format described by
// EnvelopeV1.
func encodeEnvelope(env Envelope) ([]byte, error) {
	if env.Version != EnvelopeV1 {
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedEnvelopeVersion, env.Version)
	}
	if env.KeyID == "" {
		return nil, ErrEmptyKeyID
	}
	if len(env.KeyID) > 0xFFFF {
		return nil, fmt.Errorf("encryption: key id too long to encode (%d bytes)", len(env.KeyID))
	}
	if len(env.Nonce) > 0xFF {
		return nil, fmt.Errorf("encryption: nonce too long to encode (%d bytes)", len(env.Nonce))
	}

	out := make([]byte, 0, len(envelopeMagic)+1+2+len(env.KeyID)+1+len(env.Nonce)+len(env.Ciphertext))
	out = append(out, envelopeMagic...)
	out = append(out, env.Version)

	// The two conversions below are guarded by the length checks above
	// (len(env.KeyID) <= 0xFFFF, len(env.Nonce) <= 0xFF), so neither
	// can overflow its target width.
	keyIDLen := make([]byte, 2)
	binary.BigEndian.PutUint16(keyIDLen, uint16(len(env.KeyID))) // #nosec G115 -- bounds-checked above
	out = append(out, keyIDLen...)
	out = append(out, env.KeyID...)

	out = append(out, byte(len(env.Nonce))) // #nosec G115 -- bounds-checked above
	out = append(out, env.Nonce...)

	out = append(out, env.Ciphertext...)
	return out, nil
}

// decodeEnvelope parses data produced by encodeEnvelope.
func decodeEnvelope(data []byte) (Envelope, error) {
	if len(data) < len(envelopeMagic)+1+2 {
		return Envelope{}, fmt.Errorf("%w: too short", ErrInvalidEnvelope)
	}
	if string(data[:len(envelopeMagic)]) != envelopeMagic {
		return Envelope{}, fmt.Errorf("%w: bad magic", ErrInvalidEnvelope)
	}
	offset := len(envelopeMagic)

	version := data[offset]
	offset++
	if version != EnvelopeV1 {
		return Envelope{}, fmt.Errorf("%w: %d", ErrUnsupportedEnvelopeVersion, version)
	}

	if len(data) < offset+2 {
		return Envelope{}, fmt.Errorf("%w: truncated key id length", ErrInvalidEnvelope)
	}
	keyIDLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2

	if len(data) < offset+keyIDLen {
		return Envelope{}, fmt.Errorf("%w: truncated key id", ErrInvalidEnvelope)
	}
	keyID := string(data[offset : offset+keyIDLen])
	offset += keyIDLen
	if keyID == "" {
		return Envelope{}, fmt.Errorf("%w: empty key id", ErrInvalidEnvelope)
	}

	if len(data) < offset+1 {
		return Envelope{}, fmt.Errorf("%w: truncated nonce length", ErrInvalidEnvelope)
	}
	nonceLen := int(data[offset])
	offset++

	if len(data) < offset+nonceLen {
		return Envelope{}, fmt.Errorf("%w: truncated nonce", ErrInvalidEnvelope)
	}
	nonce := data[offset : offset+nonceLen]
	offset += nonceLen

	ciphertext := data[offset:]
	if len(ciphertext) == 0 {
		return Envelope{}, fmt.Errorf("%w: empty ciphertext", ErrInvalidEnvelope)
	}

	return Envelope{
		Version:    version,
		KeyID:      keyID,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}, nil
}

// LooksLikeEnvelope reports whether data appears to be a
// this-package-produced Envelope (correct magic and a version this
// package knows), without fully validating or decrypting it. Used by
// ScanForPlaintext (registry.go) to distinguish "this field holds
// ciphertext" from "this field still holds plaintext."
func LooksLikeEnvelope(data []byte) bool {
	_, err := decodeEnvelope(data)
	return err == nil
}

// ParseEnvelope parses data (as produced by Encrypt, Decrypt's input,
// EncryptBackup, ...) into its Envelope fields without decrypting it.
// This is useful for audit/inspection tooling that needs to know which
// key ID protects a stored ciphertext -- e.g. to report which records
// still depend on a key pending rotation -- without needing access to
// the key material itself.
func ParseEnvelope(data []byte) (Envelope, error) {
	return decodeEnvelope(data)
}
