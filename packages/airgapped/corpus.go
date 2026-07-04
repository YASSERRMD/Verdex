package airgapped

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/YASSERRMD/verdex/packages/precedent"
	"github.com/YASSERRMD/verdex/packages/statute"
)

// statuteBundleSuffixes lists the file extensions ProvisionCorpus
// treats as statute corpus files within a bundle directory.
var statuteBundleSuffixes = []string{".statute.json", ".statute.txt"}

// precedentBundleSuffixes lists the file extensions ProvisionCorpus
// treats as precedent corpus files within a bundle directory.
var precedentBundleSuffixes = []string{".precedent.json", ".precedent.txt"}

// FileBundleStatuteLoader is a statute.Loader implementation that reads
// from a local directory bundle instead of a network fetch (task 4).
// It composes with packages/statute's existing loader interface and
// DefaultLoader rather than reinventing corpus parsing: FileBundleLoad
// walks the bundle directory for recognized statute files, concatenates
// their contents (each file already in one of DefaultLoader's two
// accepted shapes), and delegates the actual parsing to an inner
// statute.Loader (statute.NewDefaultLoader() if Inner is nil).
type FileBundleStatuteLoader struct {
	// Inner is the statute.Loader used to parse each bundle file's
	// content. Defaults to statute.NewDefaultLoader() when nil.
	Inner statute.Loader
}

// Ensure FileBundleStatuteLoader satisfies statute.Loader.
var _ statute.Loader = (*FileBundleStatuteLoader)(nil)

func (l *FileBundleStatuteLoader) inner() statute.Loader {
	if l.Inner != nil {
		return l.Inner
	}
	return statute.NewDefaultLoader()
}

// Load implements statute.Loader over source, which FileBundleLoad
// supplies as the concatenated content of every statute file found
// under a bundle directory. Direct callers may also use Load exactly
// like any other statute.Loader (e.g. against a single in-memory
// io.Reader) since it is a pure pass-through to the inner loader.
func (l *FileBundleStatuteLoader) Load(ctx context.Context, source io.Reader) ([]statute.RawStatute, error) {
	return l.inner().Load(ctx, source)
}

// FileBundlePrecedentLoader is the precedent.Loader analogue of
// FileBundleStatuteLoader.
type FileBundlePrecedentLoader struct {
	// Inner is the precedent.Loader used to parse each bundle file's
	// content. Defaults to precedent.NewDefaultLoader() when nil.
	Inner precedent.Loader
}

// Ensure FileBundlePrecedentLoader satisfies precedent.Loader.
var _ precedent.Loader = (*FileBundlePrecedentLoader)(nil)

func (l *FileBundlePrecedentLoader) inner() precedent.Loader {
	if l.Inner != nil {
		return l.Inner
	}
	return precedent.NewDefaultLoader()
}

// Load implements precedent.Loader.
func (l *FileBundlePrecedentLoader) Load(ctx context.Context, source io.Reader) ([]precedent.RawPrecedent, error) {
	return l.inner().Load(ctx, source)
}

// CorpusBundleResult is the outcome of ProvisionCorpus: every
// statute.RawStatute and precedent.RawPrecedent decoded from the local
// bundle, plus the file names consulted (for audit/logging).
type CorpusBundleResult struct {
	Statutes       []statute.RawStatute
	Precedents     []precedent.RawPrecedent
	StatuteFiles   []string
	PrecedentFiles []string
}

// ProvisionCorpus loads statute/precedent data from a local file-bundle
// directory (or a single file) at bundlePath rather than any network
// fetch (task 4). Every recognized statute file
// (*.statute.json/*.statute.txt) is read and parsed via
// FileBundleStatuteLoader; every recognized precedent file
// (*.precedent.json/*.precedent.txt) is read and parsed via
// FileBundlePrecedentLoader. Both loaders delegate the actual corpus
// grammar to packages/statute's and packages/precedent's existing
// DefaultLoader, so this function does not reimplement corpus parsing.
//
// Returns ErrEmptyBundlePath if bundlePath is empty, ErrBundleNotFound
// if it does not exist, and ErrEmptyCorpusBundle if the bundle
// (directory or single file) contains no recognized statute or
// precedent content.
func ProvisionCorpus(ctx context.Context, bundlePath string) (*CorpusBundleResult, error) {
	if bundlePath == "" {
		return nil, ErrEmptyBundlePath
	}
	info, err := os.Stat(bundlePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, wrapf("ProvisionCorpus", ErrBundleNotFound)
		}
		return nil, wrapf("ProvisionCorpus", err)
	}

	var files []string
	if info.IsDir() {
		entries, err := os.ReadDir(bundlePath)
		if err != nil {
			return nil, wrapf("ProvisionCorpus", err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			files = append(files, filepath.Join(bundlePath, e.Name()))
		}
	} else {
		files = []string{bundlePath}
	}
	sort.Strings(files)

	result := &CorpusBundleResult{}
	statuteLoader := &FileBundleStatuteLoader{}
	precedentLoader := &FileBundlePrecedentLoader{}

	for _, f := range files {
		name := filepath.Base(f)
		switch {
		case hasAnySuffix(name, statuteBundleSuffixes):
			data, err := os.ReadFile(f) //nolint:gosec // bundlePath is operator-supplied local path, not user-controlled input
			if err != nil {
				return nil, wrapf("ProvisionCorpus", err)
			}
			parsed, err := statuteLoader.Load(ctx, bytes.NewReader(data))
			if err != nil {
				return nil, wrapf("ProvisionCorpus", err)
			}
			result.Statutes = append(result.Statutes, parsed...)
			result.StatuteFiles = append(result.StatuteFiles, f)
		case hasAnySuffix(name, precedentBundleSuffixes):
			data, err := os.ReadFile(f) //nolint:gosec // bundlePath is operator-supplied local path, not user-controlled input
			if err != nil {
				return nil, wrapf("ProvisionCorpus", err)
			}
			parsed, err := precedentLoader.Load(ctx, bytes.NewReader(data))
			if err != nil {
				return nil, wrapf("ProvisionCorpus", err)
			}
			result.Precedents = append(result.Precedents, parsed...)
			result.PrecedentFiles = append(result.PrecedentFiles, f)
		}
	}

	if len(result.Statutes) == 0 && len(result.Precedents) == 0 {
		return nil, wrapf("ProvisionCorpus", ErrEmptyCorpusBundle)
	}
	return result, nil
}

func hasAnySuffix(name string, suffixes []string) bool {
	lower := strings.ToLower(name)
	for _, s := range suffixes {
		if strings.HasSuffix(lower, s) {
			return true
		}
	}
	return false
}
