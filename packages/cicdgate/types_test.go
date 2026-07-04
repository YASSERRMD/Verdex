package cicdgate

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func validArtifact() ReleaseArtifact {
	return ReleaseArtifact{
		Name:            "cicdgate-linux-amd64",
		DigestAlgorithm: DigestSHA256,
		Digest:          strings.Repeat("a", 64),
		SignatureState:  SignatureStatePlaceholder,
		SignatureRef:    "placeholder://sigstore/not-a-real-signature",
		BuiltAt:         time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
	}
}

func validAttestation() BuildAttestation {
	return BuildAttestation{
		SourceCommit:          strings.Repeat("b", 40),
		BuilderID:             "github-actions/phase-095-cicd-hardening",
		BuildTimestamp:        time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		InputsDigestAlgorithm: DigestSHA256,
		InputsDigest:          strings.Repeat("c", 64),
	}
}

func TestReleaseArtifact_Validate(t *testing.T) {
	t.Run("valid artifact", func(t *testing.T) {
		a := validArtifact()
		if err := a.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("nil receiver", func(t *testing.T) {
		var a *ReleaseArtifact
		if err := a.Validate(); !errors.Is(err, ErrInvalidArtifact) {
			t.Errorf("Validate() error = %v, want ErrInvalidArtifact", err)
		}
	})

	tests := []struct {
		name    string
		mutate  func(*ReleaseArtifact)
		wantErr error
	}{
		{
			name:    "blank name",
			mutate:  func(a *ReleaseArtifact) { a.Name = "  " },
			wantErr: ErrInvalidArtifact,
		},
		{
			name:    "unrecognized digest algorithm",
			mutate:  func(a *ReleaseArtifact) { a.DigestAlgorithm = "md5" },
			wantErr: ErrInvalidArtifact,
		},
		{
			name:    "digest wrong length for sha256",
			mutate:  func(a *ReleaseArtifact) { a.Digest = "abc" },
			wantErr: ErrInvalidArtifact,
		},
		{
			name:    "digest uppercase hex rejected",
			mutate:  func(a *ReleaseArtifact) { a.Digest = strings.ToUpper(strings.Repeat("a", 64)) },
			wantErr: ErrInvalidArtifact,
		},
		{
			name:    "digest non-hex characters rejected",
			mutate:  func(a *ReleaseArtifact) { a.Digest = strings.Repeat("z", 64) },
			wantErr: ErrInvalidArtifact,
		},
		{
			name:    "unrecognized signature state",
			mutate:  func(a *ReleaseArtifact) { a.SignatureState = "bogus" },
			wantErr: ErrInvalidArtifact,
		},
		{
			name: "unsigned with a signature ref set",
			mutate: func(a *ReleaseArtifact) {
				a.SignatureState = SignatureStateUnsigned
				a.SignatureRef = "should-not-be-here"
			},
			wantErr: ErrInvalidArtifact,
		},
		{
			name:    "placeholder state missing signature ref",
			mutate:  func(a *ReleaseArtifact) { a.SignatureRef = "" },
			wantErr: ErrInvalidArtifact,
		},
		{
			name:    "zero built_at",
			mutate:  func(a *ReleaseArtifact) { a.BuiltAt = time.Time{} },
			wantErr: ErrInvalidArtifact,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := validArtifact()
			tt.mutate(&a)
			err := a.Validate()
			if err == nil {
				t.Fatalf("Validate() error = nil, want wrapping %v", tt.wantErr)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() error = %v, want wrapping %v", err, tt.wantErr)
			}
		})
	}

	t.Run("unsigned artifact is valid with empty ref", func(t *testing.T) {
		a := validArtifact()
		a.SignatureState = SignatureStateUnsigned
		a.SignatureRef = ""
		if err := a.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("sha512 artifact with matching digest length", func(t *testing.T) {
		a := validArtifact()
		a.DigestAlgorithm = DigestSHA512
		a.Digest = strings.Repeat("a", 128)
		if err := a.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})
}

func TestBuildAttestation_Validate(t *testing.T) {
	t.Run("valid attestation", func(t *testing.T) {
		att := validAttestation()
		if err := att.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("nil receiver", func(t *testing.T) {
		var att *BuildAttestation
		if err := att.Validate(); !errors.Is(err, ErrInvalidAttestation) {
			t.Errorf("Validate() error = %v, want ErrInvalidAttestation", err)
		}
	})

	tests := []struct {
		name   string
		mutate func(*BuildAttestation)
	}{
		{
			name:   "source commit wrong length",
			mutate: func(a *BuildAttestation) { a.SourceCommit = "abc123" },
		},
		{
			name:   "source commit uppercase",
			mutate: func(a *BuildAttestation) { a.SourceCommit = strings.ToUpper(strings.Repeat("b", 40)) },
		},
		{
			name:   "blank builder id",
			mutate: func(a *BuildAttestation) { a.BuilderID = "" },
		},
		{
			name:   "zero build timestamp",
			mutate: func(a *BuildAttestation) { a.BuildTimestamp = time.Time{} },
		},
		{
			name:   "build timestamp far in the future",
			mutate: func(a *BuildAttestation) { a.BuildTimestamp = time.Now().Add(24 * time.Hour) },
		},
		{
			name:   "unrecognized inputs digest algorithm",
			mutate: func(a *BuildAttestation) { a.InputsDigestAlgorithm = "crc32" },
		},
		{
			name:   "inputs digest wrong length",
			mutate: func(a *BuildAttestation) { a.InputsDigest = "ab" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			att := validAttestation()
			tt.mutate(&att)
			err := att.Validate()
			if !errors.Is(err, ErrInvalidAttestation) {
				t.Errorf("Validate() error = %v, want wrapping ErrInvalidAttestation", err)
			}
		})
	}

	t.Run("build timestamp within clock skew tolerance is accepted", func(t *testing.T) {
		att := validAttestation()
		att.BuildTimestamp = time.Now().Add(1 * time.Minute)
		if err := att.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("sha512 source commit length accepted", func(t *testing.T) {
		att := validAttestation()
		att.SourceCommit = strings.Repeat("d", 64)
		if err := att.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})
}

func TestDigestAlgorithm_IsValid(t *testing.T) {
	tests := []struct {
		alg  DigestAlgorithm
		want bool
	}{
		{DigestSHA256, true},
		{DigestSHA512, true},
		{"md5", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := tt.alg.IsValid(); got != tt.want {
			t.Errorf("%q.IsValid() = %v, want %v", tt.alg, got, tt.want)
		}
	}
}

func TestSignatureState_IsValid(t *testing.T) {
	tests := []struct {
		state SignatureState
		want  bool
	}{
		{SignatureStateUnsigned, true},
		{SignatureStatePlaceholder, true},
		{SignatureStateVerified, true},
		{"bogus", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := tt.state.IsValid(); got != tt.want {
			t.Errorf("%q.IsValid() = %v, want %v", tt.state, got, tt.want)
		}
	}
}

func TestIsLowerHex(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"abc123", true},
		{"ABC123", false},
		{"", false},
		{"xyz", false},
		{"0123456789abcdef", true},
	}
	for _, tt := range tests {
		if got := isLowerHex(tt.s); got != tt.want {
			t.Errorf("isLowerHex(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}
