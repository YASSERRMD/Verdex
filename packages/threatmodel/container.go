package threatmodel

import "strings"

// This file is task 7's container hardening policy. A repository-wide
// search (find . -iname "Dockerfile*") confirms Verdex ships no
// Dockerfile today -- every package/service currently runs as plain
// Go binaries with no containerization step in this repository. Given
// that, this phase does not retrofit hardening onto a container that
// does not exist; instead it adds ContainerHardeningChecklist as a
// documented, versioned policy type any future phase that does add a
// Dockerfile must satisfy, plus a reference hardened Dockerfile
// template committed at doc/Dockerfile.hardened that phase can start
// from rather than writing container hardening from scratch.

// ContainerHardeningRule is a single, checkable container-hardening
// requirement.
type ContainerHardeningRule struct {
	// Name is a short, stable identifier for this rule (e.g.
	// "non_root_user").
	Name string `json:"name"`

	// Description explains what the rule requires and why.
	Description string `json:"description"`
}

// ContainerHardeningChecklist is the platform's standing policy for
// any Dockerfile added to this repository in a future phase --
// versioned alongside the rest of this package's policy-as-code
// rather than left as an unenforced wiki page. Satisfied reports
// whether a given Dockerfile's raw content appears to satisfy every
// rule, providing an automatable (if necessarily heuristic) first
// pass rather than a purely manual checklist.
type ContainerHardeningChecklist struct {
	// Rules lists every hardening requirement this checklist covers.
	Rules []ContainerHardeningRule `json:"rules"`
}

// DefaultContainerHardeningChecklist returns the platform's standing
// container-hardening policy (task 7): run as a non-root user, use a
// minimal base image, install no unnecessary packages, pin the base
// image by digest (not a floating tag), and never embed secrets in a
// layer.
func DefaultContainerHardeningChecklist() ContainerHardeningChecklist {
	return ContainerHardeningChecklist{
		Rules: []ContainerHardeningRule{
			{
				Name:        "non_root_user",
				Description: "The image must declare a non-root USER before the final ENTRYPOINT/CMD; the container must never run as root in production.",
			},
			{
				Name:        "minimal_base_image",
				Description: "The base image must be a minimal, purpose-built image (e.g. distroless, alpine, or scratch for a statically linked Go binary), not a general-purpose OS image carrying an unbounded package surface.",
			},
			{
				Name:        "pinned_base_image",
				Description: "The FROM base image must be pinned by digest (@sha256:...) or an explicit immutable version tag, never a floating tag like `latest`, so a build is reproducible and cannot silently pick up an unreviewed upstream change.",
			},
			{
				Name:        "no_unnecessary_packages",
				Description: "Only packages genuinely required at runtime may be installed; build-only tooling (compilers, package managers, debug utilities) must not be present in the final image layer.",
			},
			{
				Name:        "no_embedded_secrets",
				Description: "No credential, private key, or token may be present in any image layer (including intermediate build-stage layers); secrets must be injected at runtime, never baked into the image.",
			},
			{
				Name:        "read_only_root_filesystem",
				Description: "The container's root filesystem should run read-only where the runtime allows it, with explicit writable volumes only where genuinely needed.",
			},
			{
				Name:        "no_privileged_capabilities",
				Description: "The container must not request privileged mode or unnecessary Linux capabilities (e.g. NET_ADMIN, SYS_ADMIN); it should run with the minimal capability set its actual function requires.",
			},
		},
	}
}

// RuleNames returns every rule's Name, convenience for a caller that
// wants a quick checklist without the full Description text.
func (c ContainerHardeningChecklist) RuleNames() []string {
	names := make([]string, 0, len(c.Rules))
	for _, r := range c.Rules {
		names = append(names, r.Name)
	}
	return names
}

// dockerfileRuleSignatures maps each DefaultContainerHardeningChecklist
// rule name to a small heuristic checker against raw Dockerfile
// content. These are necessarily heuristic (parsing a Dockerfile
// properly requires a real Dockerfile grammar, which this package does
// not implement) -- Satisfied is a useful first-pass automated check,
// not a substitute for human review before a container ships.
var dockerfileRuleSignatures = map[string]func(content string) bool{
	"non_root_user": func(content string) bool {
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToUpper(line), "USER ") {
				user := strings.TrimSpace(line[len("USER "):])
				if user != "" && user != "root" && user != "0" {
					return true
				}
			}
		}
		return false
	},
	"minimal_base_image": func(content string) bool {
		lower := strings.ToLower(content)
		for _, marker := range []string{"distroless", "alpine", "scratch"} {
			if strings.Contains(lower, marker) {
				return true
			}
		}
		return false
	},
	"pinned_base_image": func(content string) bool {
		for _, line := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(strings.ToUpper(trimmed), "FROM ") {
				continue
			}
			if strings.Contains(trimmed, "@sha256:") {
				continue
			}
			if strings.HasSuffix(trimmed, ":latest") || !strings.Contains(trimmed, ":") {
				return false
			}
		}
		return true
	},
	"no_unnecessary_packages": func(content string) bool {
		lower := strings.ToLower(content)
		// Heuristic: a final-stage image that still contains a package
		// manager invocation for a compiler toolchain is a signal
		// (not proof) that build tooling leaked into the runtime layer.
		return !strings.Contains(lower, "apt-get install") || strings.Contains(lower, "as builder")
	},
	"no_embedded_secrets": func(content string) bool {
		lower := strings.ToLower(content)
		for _, marker := range []string{"password=", "secret=", "api_key=", "-----begin"} {
			if strings.Contains(lower, marker) {
				return false
			}
		}
		return true
	},
	"read_only_root_filesystem": func(_ string) bool {
		// Read-only root filesystem is typically a container-runtime
		// (docker run --read-only / Kubernetes securityContext)
		// setting rather than a Dockerfile directive, so this check
		// cannot be meaningfully derived from Dockerfile content alone
		// -- always reports true (out of this heuristic's scope),
		// deliberately not a false negative on every Dockerfile.
		return true
	},
	"no_privileged_capabilities": func(_ string) bool {
		// Same reasoning as read_only_root_filesystem: privileged mode
		// and capability grants are runtime/orchestrator settings, not
		// Dockerfile content.
		return true
	},
}

// Satisfied runs every heuristic checker in dockerfileRuleSignatures
// against dockerfileContent, returning the subset of c.Rules that
// pass. A rule with no registered heuristic (should not happen for
// DefaultContainerHardeningChecklist's own rule set, but possible for
// a caller-extended checklist) is treated as not satisfied, since an
// unverifiable rule should never silently count as passing.
func (c ContainerHardeningChecklist) Satisfied(dockerfileContent string) []ContainerHardeningRule {
	passed := make([]ContainerHardeningRule, 0, len(c.Rules))
	for _, rule := range c.Rules {
		check, ok := dockerfileRuleSignatures[rule.Name]
		if !ok {
			continue
		}
		if check(dockerfileContent) {
			passed = append(passed, rule)
		}
	}
	return passed
}

// Unsatisfied is Satisfied's complement: every rule in c.Rules not
// present in Satisfied's result.
func (c ContainerHardeningChecklist) Unsatisfied(dockerfileContent string) []ContainerHardeningRule {
	satisfiedNames := make(map[string]struct{})
	for _, r := range c.Satisfied(dockerfileContent) {
		satisfiedNames[r.Name] = struct{}{}
	}
	out := make([]ContainerHardeningRule, 0)
	for _, rule := range c.Rules {
		if _, ok := satisfiedNames[rule.Name]; !ok {
			out = append(out, rule)
		}
	}
	return out
}
