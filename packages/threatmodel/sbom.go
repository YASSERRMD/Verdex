package threatmodel

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// This file is task 5's dependency-pinning/SBOM generator: GenerateSBOM
// walks a repository's go.work file plus every listed package's go.mod
// to emit a structured, CycloneDX-lite Software Bill of Materials
// (module name + version per dependency, deduplicated and sorted). It
// deliberately parses go.work/go.mod with a small dependency-free
// line scanner rather than importing golang.org/x/mod/modfile: this
// repository does not otherwise depend on that module, and go.mod's
// `require (...)` block syntax is simple enough that a purpose-built
// scanner is not a maintenance risk, whereas pulling in a new external
// module purely to parse two well-known, versioned-in-this-repo file
// formats would be. A committed snapshot of this SBOM lives at
// doc/sbom.json.

// SBOMComponentType classifies an SBOMComponent, loosely mirroring
// CycloneDX's own componentType vocabulary (a small, relevant subset,
// not the full spec).
type SBOMComponentType string

const (
	// SBOMComponentApplication is this repository's own first-party
	// module (a package under packages/*, listed in go.work).
	SBOMComponentApplication SBOMComponentType = "application"

	// SBOMComponentLibrary is a third-party dependency pulled in via a
	// go.mod require directive.
	SBOMComponentLibrary SBOMComponentType = "library"
)

// SBOMComponent is a single entry in an SBOM: one module and the
// version(s) of it observed across the scanned packages.
type SBOMComponent struct {
	// Name is the module's import path (e.g.
	// "github.com/google/uuid", or
	// "github.com/YASSERRMD/verdex/packages/gateway" for a first-party
	// module).
	Name string `json:"name"`

	// Type classifies whether this is a first-party application
	// module or a third-party library.
	Type SBOMComponentType `json:"type"`

	// Versions lists every distinct version string observed for this
	// module across every scanned go.mod (sorted, deduplicated). A
	// first-party module found via go.work's use directives, which
	// carries no version, is recorded as versionless (empty
	// Versions), reflecting go.work's own "use path, not a versioned
	// dependency" semantics honestly rather than inventing a synthetic
	// version.
	Versions []string `json:"versions,omitempty"`

	// FoundIn lists which scanned package(s) (by directory name under
	// packages/) declared a require on this module -- so a caller can
	// trace exactly where a given dependency entered the graph.
	FoundIn []string `json:"found_in"`
}

// SBOM is the full generated bill of materials for a repository at a
// point in time.
type SBOM struct {
	// GeneratedAt is when this SBOM was generated.
	GeneratedAt time.Time `json:"generated_at"`

	// SchemaVersion identifies this package's own SBOM shape version,
	// not a claim of full CycloneDX spec conformance -- this is a
	// CycloneDX-lite shape (component name + version + type), not a
	// complete implementation of the CycloneDX schema.
	SchemaVersion string `json:"schema_version"`

	// Components lists every distinct module observed, first-party
	// application modules first (sorted by Name), then third-party
	// libraries (also sorted by Name).
	Components []SBOMComponent `json:"components"`
}

// sbomSchemaVersion is GenerateSBOM's own shape version, bumped if
// SBOMComponent's or SBOM's fields change in a way a consumer of a
// previously generated snapshot would need to know about.
const sbomSchemaVersion = "threatmodel-sbom-lite/1"

// GenerateSBOM walks workspaceRoot's go.work file to enumerate every
// first-party package, then parses each package's go.mod `require
// (...)` block to collect every third-party dependency and version,
// returning a deduplicated, sorted SBOM. workspaceRoot must be the
// directory containing go.work (the repository root).
func GenerateSBOM(workspaceRoot string) (SBOM, error) {
	workPath := filepath.Join(workspaceRoot, "go.work")
	packageDirs, err := parseGoWorkUseDirectives(workPath)
	if err != nil {
		return SBOM{}, wrapf("GenerateSBOM", err)
	}

	firstParty := make(map[string]*SBOMComponent)
	thirdParty := make(map[string]*SBOMComponent)

	for _, relDir := range packageDirs {
		absDir := filepath.Join(workspaceRoot, relDir)
		modPath := filepath.Join(absDir, "go.mod")

		selfModule, requires, err := parseGoMod(modPath)
		if err != nil {
			if os.IsNotExist(err) {
				// A go.work `use` entry with no go.mod yet (e.g. a
				// package directory scaffolded before its module file
				// was added) is not this function's problem to solve --
				// skip it rather than failing the whole SBOM.
				continue
			}
			return SBOM{}, wrapf("GenerateSBOM", err)
		}

		packageLabel := filepath.Base(relDir)

		if selfModule != "" {
			comp, ok := firstParty[selfModule]
			if !ok {
				comp = &SBOMComponent{Name: selfModule, Type: SBOMComponentApplication}
				firstParty[selfModule] = comp
			}
			comp.FoundIn = appendUnique(comp.FoundIn, packageLabel)
		}

		for _, req := range requires {
			if req.indirect {
				// Direct dependencies only: an SBOM's primary purpose is
				// auditing what a package actually, deliberately depends
				// on. Indirect/transitive entries are still reachable
				// via the direct dependency's own SBOM, so including
				// every transitive hop here would just duplicate that
				// information at much higher volume for no additional
				// audit value.
				continue
			}
			if strings.HasPrefix(req.path, firstPartyModulePrefix) {
				// A require on a sibling first-party package (e.g.
				// packages/compliance requiring packages/identity) is
				// recorded via that sibling's own go.work entry, not
				// re-recorded here as a third-party library.
				continue
			}
			comp, ok := thirdParty[req.path]
			if !ok {
				comp = &SBOMComponent{Name: req.path, Type: SBOMComponentLibrary}
				thirdParty[req.path] = comp
			}
			comp.Versions = appendUnique(comp.Versions, req.version)
			comp.FoundIn = appendUnique(comp.FoundIn, packageLabel)
		}
	}

	components := make([]SBOMComponent, 0, len(firstParty)+len(thirdParty))
	components = append(components, sortedComponents(firstParty)...)
	components = append(components, sortedComponents(thirdParty)...)

	return SBOM{
		GeneratedAt:   time.Now().UTC(),
		SchemaVersion: sbomSchemaVersion,
		Components:    components,
	}, nil
}

// firstPartyModulePrefix is the module-path prefix every package in
// this repository shares, used to distinguish a sibling first-party
// require from a genuine third-party library dependency.
const firstPartyModulePrefix = "github.com/YASSERRMD/verdex/packages/"

// sortedComponents returns m's values sorted by Name, with each
// component's Versions and FoundIn slices also sorted for
// deterministic output across repeated GenerateSBOM calls against an
// unchanged tree.
func sortedComponents(m map[string]*SBOMComponent) []SBOMComponent {
	out := make([]SBOMComponent, 0, len(m))
	for _, c := range m {
		sort.Strings(c.Versions)
		sort.Strings(c.FoundIn)
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// appendUnique appends v to s if it is not already present, keeping s
// small and duplicate-free without requiring the caller to maintain a
// separate set.
func appendUnique(s []string, v string) []string {
	for _, existing := range s {
		if existing == v {
			return s
		}
	}
	return append(s, v)
}

// parseGoWorkUseDirectives extracts every relative path listed inside
// a `use (...)` block (or a single-line `use ./path`) in the go.work
// file at path, e.g. "./packages/gateway" -> "packages/gateway".
func parseGoWorkUseDirectives(path string) ([]string, error) {
	f, err := os.Open(path) //nolint:gosec // path is operator-supplied (workspaceRoot/go.work), not untrusted user input.
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var dirs []string
	inUseBlock := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := stripLineComment(scanner.Text())
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue
		case strings.HasPrefix(trimmed, "use ("):
			inUseBlock = true
			continue
		case inUseBlock && trimmed == ")":
			inUseBlock = false
			continue
		case inUseBlock:
			dirs = append(dirs, normalizeUsePath(trimmed))
		case strings.HasPrefix(trimmed, "use "):
			dirs = append(dirs, normalizeUsePath(strings.TrimPrefix(trimmed, "use ")))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return dirs, nil
}

// normalizeUsePath strips a leading "./" from a go.work use-directive
// path, e.g. "./packages/gateway" -> "packages/gateway".
func normalizeUsePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "./")
	return p
}

// stripLineComment removes a trailing "// ..." line comment, mirroring
// (a small, sufficient subset of) how Go source/go.mod/go.work
// comments are written -- this does not attempt to handle a "//"
// inside a quoted string, which never occurs in a go.work use
// directive or go.mod require line in practice.
func stripLineComment(line string) string {
	if i := strings.Index(line, "//"); i >= 0 {
		return line[:i]
	}
	return line
}

// requireEntry is one parsed line from a go.mod require block.
type requireEntry struct {
	path     string
	version  string
	indirect bool
}

// parseGoMod extracts the module's own path (the `module` directive)
// and every entry in every `require (...)` block (module path,
// version, and whether it is marked `// indirect`) from the go.mod
// file at path. It intentionally ignores `replace`/`exclude`
// directives: an SBOM records what a module *declares* it depends on
// and at what version, and this repository's replace directives all
// point at sibling in-repo packages (already covered via go.work's use
// directives), never at a version substitution for a genuine
// third-party library.
func parseGoMod(path string) (selfModule string, requires []requireEntry, err error) {
	f, err := os.Open(path) //nolint:gosec // path is operator-supplied (workspaceRoot/<pkg>/go.mod), not untrusted user input.
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	inRequireBlock := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		rawLine := scanner.Text()
		indirect := strings.Contains(rawLine, "// indirect")
		line := strings.TrimSpace(stripLineComment(rawLine))
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "module "):
			selfModule = strings.TrimSpace(strings.TrimPrefix(line, "module "))
		case strings.HasPrefix(line, "require ("):
			inRequireBlock = true
		case inRequireBlock && line == ")":
			inRequireBlock = false
		case inRequireBlock:
			if entry, ok := parseRequireLine(line, indirect); ok {
				requires = append(requires, entry)
			}
		case strings.HasPrefix(line, "require "):
			// Single-line require, e.g. `require github.com/x/y v1.2.3`.
			if entry, ok := parseRequireLine(strings.TrimPrefix(line, "require "), indirect); ok {
				requires = append(requires, entry)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", nil, err
	}
	return selfModule, requires, nil
}

// parseRequireLine parses a single "<module-path> <version>" line
// (as found inside a go.mod require block) into a requireEntry. Lines
// that do not split into at least two whitespace-separated fields are
// reported via ok=false rather than an error, so one malformed or
// unexpected line does not abort parsing the rest of the file.
func parseRequireLine(line string, indirect bool) (requireEntry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return requireEntry{}, false
	}
	return requireEntry{path: fields[0], version: fields[1], indirect: indirect}, true
}

// WriteSBOMSnapshot generates an SBOM for workspaceRoot and writes it
// as indented JSON to destPath, overwriting any existing file --
// callers regenerating the committed doc/sbom.json snapshot use this
// directly rather than hand-assembling the marshal/write themselves.
func WriteSBOMSnapshot(workspaceRoot, destPath string) error {
	sbom, err := GenerateSBOM(workspaceRoot)
	if err != nil {
		return wrapf("WriteSBOMSnapshot", err)
	}
	data, err := json.MarshalIndent(sbom, "", "  ")
	if err != nil {
		return wrapf("WriteSBOMSnapshot", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(destPath, data, 0o644); err != nil { // #nosec G306 -- SBOM snapshot is a non-sensitive, repo-committed artifact. //nolint:gosec
		return wrapf("WriteSBOMSnapshot", err)
	}
	return nil
}

// countPackageDirs is a small helper reported in an SBOM's
// human-readable summary (SummarizeSBOM below), counting how many
// distinct packages contributed at least one FoundIn entry across
// components.
func countPackageDirs(sbom SBOM) int {
	seen := make(map[string]struct{})
	for _, c := range sbom.Components {
		for _, pkg := range c.FoundIn {
			seen[pkg] = struct{}{}
		}
	}
	return len(seen)
}

// SummarizeSBOM renders a short, human-readable one-line summary of
// sbom, e.g. "42 components (5 application, 37 library) across 12
// packages, generated 2026-07-04T00:00:00Z".
func SummarizeSBOM(sbom SBOM) string {
	var appCount, libCount int
	for _, c := range sbom.Components {
		switch c.Type {
		case SBOMComponentApplication:
			appCount++
		case SBOMComponentLibrary:
			libCount++
		}
	}
	return fmt.Sprintf("%d components (%d application, %d library) across %d packages, generated %s",
		len(sbom.Components), appCount, libCount, countPackageDirs(sbom), sbom.GeneratedAt.Format(time.RFC3339))
}
