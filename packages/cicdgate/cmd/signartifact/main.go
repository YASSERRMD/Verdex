// Command signartifact is the CI entry point for tasks 4-5's signed
// build and provenance/attestation step. It reads a file from disk,
// computes its SHA-256 digest, builds a cicdgate.ReleaseArtifact
// (with SignatureState set to the placeholder state -- see
// doc/cicd.md's "What is simulated/placeholder versus real": this
// environment has no real Sigstore/cosign signing identity
// configured) and a cicdgate.BuildAttestation from the supplied
// source commit and builder identity, runs cicdgate.Verify against
// the pair, and prints the resulting JSON records to stdout so a CI
// step can archive them alongside the built artifact.
//
// Run from the repository root as:
//
//	go run ./packages/cicdgate/cmd/signartifact <artifact-path> <source-commit> <builder-id>
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/YASSERRMD/verdex/packages/cicdgate"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: signartifact <artifact-path> <source-commit> <builder-id>")
		os.Exit(2)
	}

	artifactPath := os.Args[1]
	sourceCommit := os.Args[2]
	builderID := os.Args[3]

	digest, err := sha256Hex(artifactPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "signartifact: reading %s: %v\n", artifactPath, err)
		os.Exit(1)
	}

	inputsDigest, err := sha256Hex(artifactPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "signartifact: computing inputs digest: %v\n", err)
		os.Exit(1)
	}

	now := time.Now().UTC()

	artifact := cicdgate.ReleaseArtifact{
		Name:            artifactPath,
		DigestAlgorithm: cicdgate.DigestSHA256,
		Digest:          digest,
		SignatureState:  cicdgate.SignatureStatePlaceholder,
		SignatureRef:    "placeholder://sigstore/" + artifactPath,
		BuiltAt:         now,
	}

	attestation := cicdgate.BuildAttestation{
		SourceCommit:          sourceCommit,
		BuilderID:             builderID,
		BuildTimestamp:        now,
		InputsDigestAlgorithm: cicdgate.DigestSHA256,
		InputsDigest:          inputsDigest,
	}

	if err := cicdgate.Verify(&artifact, &attestation); err != nil {
		fmt.Fprintln(os.Stderr, "signartifact: Verify:", err)
		os.Exit(1)
	}

	out := struct {
		Artifact    cicdgate.ReleaseArtifact  `json:"artifact"`
		Attestation cicdgate.BuildAttestation `json:"attestation"`
	}{
		Artifact:    artifact,
		Attestation: attestation,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, "signartifact: encoding output:", err)
		os.Exit(1)
	}
}

// sha256Hex returns the hex-encoded SHA-256 digest of the file at
// path.
func sha256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
