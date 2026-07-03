package encryption

import "context"

// EncryptBackup wraps an arbitrary backup blob (e.g. a database dump,
// a case export archive) in the same versioned Envelope format
// Encrypt uses for individual fields. It is a distinct entry point
// from Encrypt purely for call-site clarity -- "this is a whole backup
// artifact, not a single sensitive field" -- and to give backup
// tooling a name to grep for; the underlying mechanism (AES-256-GCM,
// random nonce per call, key-ID-tagged envelope) is identical.
func EncryptBackup(ctx context.Context, source KeySource, backup []byte) ([]byte, error) {
	return Encrypt(ctx, source, backup)
}

// DecryptBackup reverses EncryptBackup.
func DecryptBackup(ctx context.Context, source KeySource, envelope []byte) ([]byte, error) {
	return Decrypt(ctx, source, envelope)
}
