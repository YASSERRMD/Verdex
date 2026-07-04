package airgapped_test

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/airgapped"
	"github.com/YASSERRMD/verdex/packages/dataresidency"
	"github.com/YASSERRMD/verdex/packages/keymanagement"
	"github.com/YASSERRMD/verdex/packages/router"
)

// stringsReader is a tiny helper so test bodies can pass a literal
// string wherever an io.Reader is expected.
func stringsReader(s string) io.Reader {
	return strings.NewReader(s)
}

// marshalManifest JSON-encodes an airgapped.UpdateManifest for writing
// to a test bundle's manifest.json.
func marshalManifest(t *testing.T, m airgapped.UpdateManifest) []byte {
	t.Helper()
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("json.Marshal(manifest): %v", err)
	}
	return data
}

// roundTripJSON marshals v to JSON and back, failing the test if
// either step errors. It is used to assert that this package's report
// types are safely JSON-serializable end to end.
func roundTripJSON(t *testing.T, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
}

// tamperManifestChecksum flips one hex character in the first checksum
// value found in a marshaled UpdateManifest's raw JSON, without
// re-signing, so a test can assert that ApplyUpdateBundle rejects a
// manifest whose signed payload no longer matches its signature.
func tamperManifestChecksum(t *testing.T, raw []byte) []byte {
	t.Helper()
	var m airgapped.UpdateManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("json.Unmarshal(manifest): %v", err)
	}
	for k, v := range m.Files {
		if len(v) == 0 {
			continue
		}
		flipped := "0"
		if v[0] == '0' {
			flipped = "1"
		}
		m.Files[k] = flipped + v[1:]
		break
	}
	return marshalManifest(t, m)
}

// newTestKeyProvider builds a real keymanagement.FileProvider rooted at
// a fresh temp directory, mirroring packages/keymanagement's own test
// convention for constructing a FileProvider without a database.
func newTestKeyProvider(t *testing.T) *keymanagement.FileProvider {
	t.Helper()
	root := t.TempDir()
	masterKey := keymanagement.DeriveMasterKey("air-gapped-test-passphrase")
	fp, err := keymanagement.NewFileProvider(root, masterKey, keymanagement.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("NewFileProvider: %v", err)
	}
	return fp
}

// validProfile builds a Profile that passes Validate: an air-gapped
// residency preset, an air-gapped routing policy, and a real
// FileProvider.
func validProfile(t *testing.T) *airgapped.Profile {
	t.Helper()
	deploymentID := uuid.New()
	residency := dataresidency.AirGappedPreset(deploymentID)
	routing := router.RoutingPolicy{
		TaskRoutes:      map[router.TaskType][]string{router.TaskChat: {"local:llama3"}},
		FallbackChain:   []string{"local:llama3"},
		TenantOverrides: map[string]map[router.TaskType][]string{},
		AirGappedOnly:   true,
	}
	return airgapped.NewProfile(deploymentID, residency, routing, newTestKeyProvider(t), []string{"192.168.1.50:11434"})
}
