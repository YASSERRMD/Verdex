package iac

import (
	"fmt"
	"strings"
)

// checkComposeServices validates a docker-compose "services:" document
// against tier's rules: every service's "environment:" must reference
// secrets via "env://VAR_NAME" (packages/config/secrets.go's
// convention), never a literal-looking password/DSN value, and (for
// TierAirgapped only) every "image:" value must be digest-pinned
// against an offline registry host, never a public tag.
func checkComposeServices(tier Tier, doc map[string]any) []ManifestCheckResult {
	services, _ := doc["services"].(map[string]any)
	if len(services) == 0 {
		return []ManifestCheckResult{{
			Kind: "compose_services_present", Passed: false,
			Detail: "compose document has no services",
		}}
	}

	var checks []ManifestCheckResult
	checks = append(checks, ManifestCheckResult{Kind: "compose_services_present", Passed: true})

	for name, raw := range services {
		svc, _ := raw.(map[string]any)
		if svc == nil {
			continue
		}
		checks = append(checks, checkComposeSecretReferences(name, svc)...)
		if tier == TierAirgapped {
			checks = append(checks, checkComposeImageDigestPinned(name, svc))
		}
	}
	return checks
}

// checkComposeSecretReferences requires every environment value whose
// key looks secret-shaped (DSN/PASSWORD/KEY/SECRET/TOKEN) to be an
// "env://" (or "vault://") reference, matching
// packages/config/secrets.go's scheme convention -- never a literal
// value committed to this file.
func checkComposeSecretReferences(serviceName string, svc map[string]any) []ManifestCheckResult {
	env, _ := svc["environment"].(map[string]any)
	if len(env) == 0 {
		return nil
	}

	var checks []ManifestCheckResult
	for key, val := range env {
		if !looksLikeSecretKey(key) {
			continue
		}
		str, ok := val.(string)
		passed := ok && (strings.HasPrefix(str, "env://") || strings.HasPrefix(str, "vault://"))
		checks = append(checks, ManifestCheckResult{
			Kind:   "secret_reference_not_literal",
			Passed: passed,
			Detail: fmt.Sprintf("service %q env %q", serviceName, key),
		})
	}
	return checks
}

// looksLikeSecretKey reports whether an environment variable name
// suggests it carries sensitive material. It matches on a trailing
// word (e.g. "...ENCRYPTION_KEY", "...DATABASE_PASSWORD") rather than
// a raw substring, so a selector-style name that merely mentions one
// of these words without being one -- e.g. "VERDEX_KEY_PROVIDER" (a
// provider-selector string, not a credential; see
// infra/airgapped/docker-compose.yml) -- is not mistaken for a secret
// reference.
func looksLikeSecretKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, marker := range []string{"PASSWORD", "DSN", "KEY", "SECRET", "TOKEN"} {
		if upper == marker || strings.HasSuffix(upper, "_"+marker) {
			return true
		}
	}
	return false
}

// checkComposeImageDigestPinned requires an air-gapped-tier service's
// "image:" value to match digestPinnedImage -- an offline registry
// host plus a sha256 digest, never a public tag such as "postgres:16"
// or "gateway:latest".
func checkComposeImageDigestPinned(serviceName string, svc map[string]any) ManifestCheckResult {
	image, _ := svc["image"].(string)
	if image == "" {
		return ManifestCheckResult{
			Kind: "airgapped_image_digest_pinned", Passed: false,
			Detail: fmt.Sprintf("service %q has no image", serviceName),
		}
	}
	if !digestPinnedImage.MatchString(image) {
		return ManifestCheckResult{
			Kind: "airgapped_image_digest_pinned", Passed: false,
			Detail: fmt.Sprintf("service %q image %q is not pinned by sha256 digest against an offline registry host", serviceName, image),
		}
	}
	return ManifestCheckResult{Kind: "airgapped_image_digest_pinned", Passed: true}
}

// checkK8sDeployment validates a Kubernetes Deployment document:
// containers must run non-root with a read-only root filesystem and
// no Linux capabilities (packages/threatmodel/doc/Dockerfile.hardened's
// runtime-settings notes), and, for TierAirgapped, every container
// image must be digest-pinned and imagePullPolicy must be "Never".
func checkK8sDeployment(tier Tier, doc map[string]any) []ManifestCheckResult {
	var checks []ManifestCheckResult

	containers := extractContainers(doc)
	if len(containers) == 0 {
		return []ManifestCheckResult{{
			Kind: "deployment_has_containers", Passed: false,
			Detail: "Deployment spec.template.spec.containers is empty or missing",
		}}
	}
	checks = append(checks, ManifestCheckResult{Kind: "deployment_has_containers", Passed: true})

	for _, c := range containers {
		name, _ := c["name"].(string)
		checks = append(checks, checkContainerHardening(name, c)...)
		if tier == TierAirgapped {
			checks = append(checks, checkContainerAirgappedImage(name, c)...)
		}
	}
	return checks
}

// extractContainers walks a Deployment document's
// spec.template.spec.containers list and returns each container as a
// generic map.
func extractContainers(doc map[string]any) []map[string]any {
	spec, _ := doc["spec"].(map[string]any)
	template, _ := spec["template"].(map[string]any)
	podSpec, _ := template["spec"].(map[string]any)
	rawContainers, _ := podSpec["containers"].([]any)

	var out []map[string]any
	for _, rc := range rawContainers {
		if c, ok := rc.(map[string]any); ok {
			out = append(out, c)
		}
	}
	return out
}

// checkContainerHardening requires securityContext to disable
// privilege escalation, force a read-only root filesystem, and drop
// all capabilities, mirroring
// packages/threatmodel/doc/Dockerfile.hardened's own
// read_only_root_filesystem/no_privileged_capabilities notes.
func checkContainerHardening(containerName string, c map[string]any) []ManifestCheckResult {
	sc, _ := c["securityContext"].(map[string]any)
	if sc == nil {
		return []ManifestCheckResult{{
			Kind: "container_hardening_present", Passed: false,
			Detail: fmt.Sprintf("container %q has no securityContext", containerName),
		}}
	}

	var checks []ManifestCheckResult

	allowEsc, ok := sc["allowPrivilegeEscalation"].(bool)
	checks = append(checks, ManifestCheckResult{
		Kind:   "no_privilege_escalation",
		Passed: ok && !allowEsc,
		Detail: fmt.Sprintf("container %q allowPrivilegeEscalation", containerName),
	})

	readOnly, ok := sc["readOnlyRootFilesystem"].(bool)
	checks = append(checks, ManifestCheckResult{
		Kind:   "read_only_root_filesystem",
		Passed: ok && readOnly,
		Detail: fmt.Sprintf("container %q readOnlyRootFilesystem", containerName),
	})

	capabilities, _ := sc["capabilities"].(map[string]any)
	dropped, _ := capabilities["drop"].([]any)
	droppedAll := false
	for _, d := range dropped {
		if s, ok := d.(string); ok && s == "ALL" {
			droppedAll = true
		}
	}
	checks = append(checks, ManifestCheckResult{
		Kind:   "capabilities_dropped_all",
		Passed: droppedAll,
		Detail: fmt.Sprintf("container %q capabilities.drop", containerName),
	})

	return checks
}

// checkContainerAirgappedImage requires an air-gapped-tier container's
// image to be digest-pinned and its imagePullPolicy to be "Never" --
// this cluster has no route to any registry outside the offline
// mirror, so relying on a default pull policy is a latent failure.
func checkContainerAirgappedImage(containerName string, c map[string]any) []ManifestCheckResult {
	image, _ := c["image"].(string)
	imagePinned := digestPinnedImage.MatchString(image)

	pullPolicy, _ := c["imagePullPolicy"].(string)

	return []ManifestCheckResult{
		{
			Kind:   "airgapped_image_digest_pinned",
			Passed: imagePinned,
			Detail: fmt.Sprintf("container %q image %q", containerName, image),
		},
		{
			Kind:   "airgapped_image_pull_policy_never",
			Passed: pullPolicy == "Never",
			Detail: fmt.Sprintf("container %q imagePullPolicy %q", containerName, pullPolicy),
		},
	}
}

// checkK8sConfigMap validates a ConfigMap document: it must carry
// VERDEX_PROFILE matching tier, must carry a data-residency region key
// only when tier is TierCloud, and must never carry one otherwise
// (packages/iac.DeploymentProfile.Validate's same cloud-only-region
// rule, checked here at the manifest layer too).
func checkK8sConfigMap(tier Tier, doc map[string]any) []ManifestCheckResult {
	data, _ := doc["data"].(map[string]any)
	if data == nil {
		return []ManifestCheckResult{{
			Kind: "configmap_has_data", Passed: false,
			Detail: "ConfigMap has no data section",
		}}
	}

	var checks []ManifestCheckResult
	checks = append(checks, ManifestCheckResult{Kind: "configmap_has_data", Passed: true})

	profile, _ := data["VERDEX_PROFILE"].(string)
	checks = append(checks, ManifestCheckResult{
		Kind:   "configmap_profile_matches_tier",
		Passed: profile == string(tier),
		Detail: fmt.Sprintf("VERDEX_PROFILE=%q, tier=%q", profile, tier),
	})

	_, hasRegion := data["VERDEX_DATARESIDENCY_REGION"]
	switch tier {
	case TierCloud:
		checks = append(checks, ManifestCheckResult{
			Kind: "cloud_region_present", Passed: hasRegion,
			Detail: "TierCloud ConfigMap must declare VERDEX_DATARESIDENCY_REGION",
		})
	case TierOnPrem, TierAirgapped:
		checks = append(checks, ManifestCheckResult{
			Kind: "non_cloud_region_absent", Passed: !hasRegion,
			Detail: fmt.Sprintf("%s ConfigMap must not declare VERDEX_DATARESIDENCY_REGION", tier),
		})
	}

	return checks
}

// checkK8sService validates a Service document has at least one named
// port -- infra/*/deployment.yaml's containerPort names
// (targetPort references) must resolve to something.
func checkK8sService(doc map[string]any) []ManifestCheckResult {
	spec, _ := doc["spec"].(map[string]any)
	ports, _ := spec["ports"].([]any)
	if len(ports) == 0 {
		return []ManifestCheckResult{{
			Kind: "service_has_ports", Passed: false,
			Detail: "Service spec.ports is empty or missing",
		}}
	}
	return []ManifestCheckResult{{Kind: "service_has_ports", Passed: true}}
}

// checkK8sPVC validates a PersistentVolumeClaim document. Only
// TierOnPrem and TierAirgapped are expected to declare one at all
// (TierCloud defers to a managed database instead -- see
// infra/cloud/docker-compose.yml's own comment on this point), and
// when they do, storageClassName must name a local storage class
// ("local-path" by this phase's convention), never a cloud elastic
// storage class.
func checkK8sPVC(tier Tier, doc map[string]any) []ManifestCheckResult {
	if tier == TierCloud {
		return []ManifestCheckResult{{
			Kind: "cloud_tier_has_no_pvc", Passed: false,
			Detail: "TierCloud defers to a managed database and should not declare a PersistentVolumeClaim",
		}}
	}

	spec, _ := doc["spec"].(map[string]any)
	storageClass, _ := spec["storageClassName"].(string)
	isLocal := storageClass != "" && !strings.Contains(strings.ToLower(storageClass), "cloud")

	return []ManifestCheckResult{{
		Kind:   "pvc_uses_local_storage_class",
		Passed: isLocal,
		Detail: fmt.Sprintf("storageClassName=%q", storageClass),
	}}
}

// checkAirgapComposition validates the non-Kubernetes
// profile-composition.yaml structural cross-reference: it must carry
// every field infra/airgapped/profile-composition.yaml's own header
// comment documents, and only applies to TierAirgapped.
func checkAirgapComposition(tier Tier, doc map[string]any) []ManifestCheckResult {
	if tier != TierAirgapped {
		return []ManifestCheckResult{{
			Kind: "airgap_composition_only_for_airgapped_tier", Passed: false,
			Detail: fmt.Sprintf("profile-composition document evaluated against non-airgapped tier %q", tier),
		}}
	}

	required := []string{
		"config_profile_name_ref",
		"residency_preset_ref",
		"routing_air_gapped_only_ref",
		"key_provider_ref",
		"profile_deployment_id_ref",
		"offline_registry_host",
	}

	var checks []ManifestCheckResult
	for _, field := range required {
		val, present := doc[field]
		nonEmpty := present && !isEmptyYAMLValue(val)
		checks = append(checks, ManifestCheckResult{
			Kind:   "airgap_composition_field_present",
			Passed: nonEmpty,
			Detail: field,
		})
	}
	return checks
}

// isEmptyYAMLValue reports whether a decoded YAML scalar is
// effectively blank (nil, or an empty/whitespace-only string).
func isEmptyYAMLValue(v any) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	return false
}
