package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"
)

// Secret reference scheme prefixes recognized in string config fields.
const (
	// envSecretScheme resolves to the value of an environment
	// variable, e.g. "env://VERDEX_DATABASE_PASSWORD".
	envSecretScheme = "env://"

	// vaultSecretScheme resolves via a pluggable SecretResolver, e.g.
	// "vault://secret/data/verdex#dsn". No real Vault integration
	// exists yet in this phase; see VaultResolver below.
	vaultSecretScheme = "vault://"
)

// SecretResolver resolves a single secret reference of a given scheme
// (without the "scheme://" prefix) to its plaintext value. Resolvers
// are looked up by scheme name ("env" or "vault").
type SecretResolver interface {
	// Resolve returns the plaintext value referenced by ref, where ref
	// is everything after "scheme://" in the original reference
	// string (e.g. for "env://FOO" the env resolver receives "FOO";
	// for "vault://path#key" the vault resolver receives "path#key").
	Resolve(ref string) (string, error)
}

// SecretResolverFunc adapts a function to the SecretResolver interface.
type SecretResolverFunc func(ref string) (string, error)

// Resolve implements SecretResolver.
func (f SecretResolverFunc) Resolve(ref string) (string, error) {
	return f(ref)
}

// envResolver resolves "env://VAR_NAME" references against the
// process environment. It fails loudly (returns an error) if the
// referenced variable is unset, since a silently-empty secret is
// almost always a deployment mistake.
type envResolver struct{}

func (envResolver) Resolve(ref string) (string, error) {
	val, ok := os.LookupEnv(ref)
	if !ok {
		return "", fmt.Errorf("env secret reference %q: environment variable %s is not set", envSecretScheme+ref, ref)
	}
	return val, nil
}

// VaultResolver is a placeholder SecretResolver for "vault://path#key"
// references. It is NOT wired to a real HashiCorp Vault (or any other
// secret store) yet -- that integration is out of scope for this
// phase. Calling Resolve always returns an error explaining that the
// backend is unimplemented, so a vault:// reference fails loudly
// rather than silently producing an empty secret.
type VaultResolver struct{}

// Resolve implements SecretResolver. It always errors: there is no
// real Vault backend wired up in this phase.
func (VaultResolver) Resolve(ref string) (string, error) {
	return "", fmt.Errorf("vault secret reference %q: no Vault backend is configured (placeholder resolver; not implemented)", vaultSecretScheme+ref)
}

// multiResolver dispatches to a scheme-specific SecretResolver based
// on the reference's "scheme://" prefix.
type multiResolver struct {
	env   SecretResolver
	vault SecretResolver
}

// NewDefaultResolver returns the SecretResolver used by Loader unless
// overridden via WithSecretResolver. It resolves env:// references
// against the process environment and vault:// references via the
// placeholder VaultResolver (which always errors).
func NewDefaultResolver() SecretResolver {
	return &multiResolver{
		env:   envResolver{},
		vault: VaultResolver{},
	}
}

// Resolve implements SecretResolver by dispatching on ref's scheme.
// ref must include the "scheme://" prefix here (unlike the
// scheme-specific resolvers, which receive it stripped), since
// multiResolver is the entry point that knows how to route by scheme.
func (m *multiResolver) Resolve(ref string) (string, error) {
	switch {
	case strings.HasPrefix(ref, envSecretScheme):
		return m.env.Resolve(strings.TrimPrefix(ref, envSecretScheme))
	case strings.HasPrefix(ref, vaultSecretScheme):
		return m.vault.Resolve(strings.TrimPrefix(ref, vaultSecretScheme))
	default:
		return "", fmt.Errorf("unrecognized secret reference scheme in %q", ref)
	}
}

// isSecretReference reports whether s looks like a secret reference
// this package knows how to resolve.
func isSecretReference(s string) bool {
	return strings.HasPrefix(s, envSecretScheme) || strings.HasPrefix(s, vaultSecretScheme)
}

// resolveSecrets walks every string field in cfg and, for any value
// that looks like a secret reference (env:// or vault://), replaces it
// in place with the resolved plaintext using resolver. It returns the
// first resolution error encountered, wrapped with field context.
func resolveSecrets(cfg *Config, resolver SecretResolver) error {
	return resolveSecretsInStruct(reflect.ValueOf(cfg).Elem(), resolver)
}

func resolveSecretsInStruct(v reflect.Value, resolver SecretResolver) error {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		fieldValue := v.Field(i)

		switch fieldValue.Kind() {
		case reflect.Struct:
			if err := resolveSecretsInStruct(fieldValue, resolver); err != nil {
				return err
			}
		case reflect.String:
			raw := fieldValue.String()
			if !isSecretReference(raw) {
				continue
			}
			resolved, err := resolver.Resolve(raw)
			if err != nil {
				return fmt.Errorf("config: resolve secret for field %s: %w", field.Name, err)
			}
			fieldValue.SetString(resolved)
		default:
			// Other kinds (int, bool, slice, etc.) never carry secret
			// references and are intentionally left untouched.
		}
	}

	return nil
}
