package iac

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ManifestKind classifies what a validated YAML document represents,
// so a ValidationReport can enumerate exactly what was inspected and
// what failed, mirroring packages/dataresidency.CheckKind/
// packages/airgapped.ConformanceCheckKind's per-check enumeration
// convention.
type ManifestKind string

const (
	// ManifestKindComposeService is a docker-compose.yml document (a
	// "services:" map at the top level).
	ManifestKindComposeService ManifestKind = "compose_service"

	// ManifestKindK8sConfigMap is a Kubernetes ConfigMap document.
	ManifestKindK8sConfigMap ManifestKind = "k8s_configmap"

	// ManifestKindK8sDeployment is a Kubernetes Deployment document.
	ManifestKindK8sDeployment ManifestKind = "k8s_deployment"

	// ManifestKindK8sService is a Kubernetes Service document.
	ManifestKindK8sService ManifestKind = "k8s_service"

	// ManifestKindK8sPVC is a Kubernetes PersistentVolumeClaim document.
	ManifestKindK8sPVC ManifestKind = "k8s_persistentvolumeclaim"

	// ManifestKindAirgapComposition is the non-Kubernetes
	// profile-composition.yaml structural cross-reference described in
	// infra/airgapped/profile-composition.yaml's own header comment.
	ManifestKindAirgapComposition ManifestKind = "airgap_profile_composition"

	// ManifestKindUnknown is returned for a recognized-YAML document
	// whose shape does not match any of the kinds above. Structural
	// per-tier rules are skipped for it (it is still checked for parsing
	// only), rather than the file being rejected outright -- a caller
	// exploring an infra/ directory it did not fully anticipate should
	// see "not evaluated" instead of a false failure.
	ManifestKindUnknown ManifestKind = "unknown"
)

// ManifestCheckResult is the outcome of a single structural rule
// ValidateManifest evaluated against one document within the target
// file.
type ManifestCheckResult struct {
	Kind   string `json:"kind"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

// ValidationReport is the result of ValidateManifest: a point-in-time
// structural assessment of one manifest file against its Tier's
// rules, mirroring packages/airgapped.ConformanceReport/
// packages/dataresidency.Report's Passed()/Failures() shape exactly.
type ValidationReport struct {
	Tier          Tier                  `json:"tier"`
	ManifestPath  string                `json:"manifest_path"`
	GeneratedAt   time.Time             `json:"generated_at"`
	DocumentKinds []ManifestKind        `json:"document_kinds,omitempty"`
	Checks        []ManifestCheckResult `json:"checks"`
}

// Passed reports whether every check in r succeeded. A report with no
// checks is considered not passed -- ValidateManifest never returns an
// empty, vacuously-true report, matching
// packages/airgapped.ConformanceReport.Passed's fail-closed
// convention.
func (r *ValidationReport) Passed() bool {
	if r == nil || len(r.Checks) == 0 {
		return false
	}
	for _, c := range r.Checks {
		if !c.Passed {
			return false
		}
	}
	return true
}

// Failures returns the subset of r.Checks that did not pass.
func (r *ValidationReport) Failures() []ManifestCheckResult {
	if r == nil {
		return nil
	}
	var out []ManifestCheckResult
	for _, c := range r.Checks {
		if !c.Passed {
			out = append(out, c)
		}
	}
	return out
}

// digestPinnedImage matches "<registry-host>/<name>@sha256:<64-hex-digest>",
// the only image-reference shape infra/airgapped's manifests are
// allowed to use (see infra/airgapped/profile-composition.yaml's
// offline_registry_host field).
var digestPinnedImage = regexp.MustCompile(`^[a-zA-Z0-9.-]+/[a-zA-Z0-9._/-]+@sha256:[a-f0-9]{64}$`)

// ValidateManifest does real structural validation of the YAML file at
// manifestPath against tier's rules: it parses the file as YAML (every
// document, if it is a multi-document stream), classifies each
// document's ManifestKind, and checks the required fields/sections
// this phase's own infra/<tier>/ manifests are expected to carry. It
// is deliberately not a Kubernetes schema validator (no apiserver
// dry-run, no CRD awareness) -- it checks the specific structural
// invariants this phase's design calls for per tier: e.g. an
// air-gapped manifest's every "image:" value must be digest-pinned
// against an offline registry host, never a public tag; a cloud
// manifest declaring a ConfigMap should carry a data-residency region
// key; an on-prem manifest must not.
//
// Returns ErrEmptyManifestPath if manifestPath is blank,
// ErrInvalidTier if tier is not recognized, ErrManifestNotFound if the
// file cannot be read, and ErrManifestNotYAML if its content does not
// parse as YAML at all. A structural rule failing is NOT a Go error --
// it is recorded as a failed ManifestCheckResult in the returned
// ValidationReport, exactly as
// packages/dataresidency.Verify/packages/airgapped.Conformance return
// a report whose Passed() may be false without the call itself
// erroring.
func ValidateManifest(tier Tier, manifestPath string) (ValidationReport, error) {
	if strings.TrimSpace(manifestPath) == "" {
		return ValidationReport{}, ErrEmptyManifestPath
	}
	if !tier.IsValid() {
		return ValidationReport{}, ErrInvalidTier
	}

	raw, err := os.ReadFile(manifestPath) //nolint:gosec // manifestPath is an operator-supplied infra/ file path, not untrusted user input (mirrors .golangci.yml's G304 repo-wide exclusion rationale)
	if err != nil {
		return ValidationReport{}, wrapf("ValidateManifest", fmt.Errorf("%w: %s: %v", ErrManifestNotFound, manifestPath, err))
	}

	docs, err := decodeYAMLDocuments(raw)
	if err != nil {
		return ValidationReport{}, wrapf("ValidateManifest", fmt.Errorf("%w: %s: %v", ErrManifestNotYAML, manifestPath, err))
	}

	report := ValidationReport{
		Tier:         tier,
		ManifestPath: manifestPath,
		GeneratedAt:  time.Now().UTC(),
	}

	if len(docs) == 0 {
		report.Checks = append(report.Checks, ManifestCheckResult{
			Kind: "non_empty_document", Passed: false,
			Detail: "manifest parsed as YAML but contained no documents",
		})
		return report, nil
	}

	for _, doc := range docs {
		kind := classifyManifest(doc)
		report.DocumentKinds = append(report.DocumentKinds, kind)
		report.Checks = append(report.Checks, checksForDocument(tier, kind, doc)...)
	}

	return report, nil
}

// decodeYAMLDocuments parses raw as a (possibly multi-document) YAML
// stream and returns every non-null document as a generic map. An
// empty document (e.g. a trailing "---" with nothing after it) is
// skipped rather than reported, matching yaml.v3's own treatment of
// blank documents.
func decodeYAMLDocuments(raw []byte) ([]map[string]any, error) {
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	var docs []map[string]any
	for {
		var doc map[string]any
		err := dec.Decode(&doc)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
		if doc == nil {
			continue
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

// classifyManifest inspects a decoded YAML document's shape and
// returns the ManifestKind it structurally matches.
func classifyManifest(doc map[string]any) ManifestKind {
	if _, ok := doc["services"]; ok {
		return ManifestKindComposeService
	}
	if _, ok := doc["offline_registry_host"]; ok {
		return ManifestKindAirgapComposition
	}
	kindVal, _ := doc["kind"].(string)
	switch kindVal {
	case "ConfigMap":
		return ManifestKindK8sConfigMap
	case "Deployment":
		return ManifestKindK8sDeployment
	case "Service":
		return ManifestKindK8sService
	case "PersistentVolumeClaim":
		return ManifestKindK8sPVC
	}
	return ManifestKindUnknown
}

// checksForDocument dispatches to the per-kind, per-tier rule
// functions and returns every ManifestCheckResult they produce.
func checksForDocument(tier Tier, kind ManifestKind, doc map[string]any) []ManifestCheckResult {
	var checks []ManifestCheckResult

	switch kind {
	case ManifestKindComposeService:
		checks = append(checks, checkComposeServices(tier, doc)...)
	case ManifestKindK8sDeployment:
		checks = append(checks, checkK8sDeployment(tier, doc)...)
	case ManifestKindK8sConfigMap:
		checks = append(checks, checkK8sConfigMap(tier, doc)...)
	case ManifestKindK8sService:
		checks = append(checks, checkK8sService(doc)...)
	case ManifestKindK8sPVC:
		checks = append(checks, checkK8sPVC(tier, doc)...)
	case ManifestKindAirgapComposition:
		checks = append(checks, checkAirgapComposition(tier, doc)...)
	case ManifestKindUnknown:
		checks = append(checks, ManifestCheckResult{
			Kind: "recognized_shape", Passed: true,
			Detail: "document shape not evaluated by any tier-specific rule (parsed successfully)",
		})
	}
	return checks
}
