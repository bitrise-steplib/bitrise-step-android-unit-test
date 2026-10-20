package testattachment

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/fileutil"
)

var (
	// ErrTrackedByGit is returned for a file that is tracked by git, e.g. a committed reference image.
	ErrTrackedByGit = errors.New("file is tracked by git")
	// ErrDuplicateName is returned when more than one collected file has the same name.
	ErrDuplicateName = errors.New("more than one file has this name")
	// ErrAlreadyExported is returned for a file that an earlier step already exported unchanged.
	ErrAlreadyExported = errors.New("file was already exported by an earlier step")
	// ErrDuplicateCopy is returned for a file that has the same name and size as another collected file,
	// or as a file an earlier step already exported.
	ErrDuplicateCopy = errors.New("copy of another file with the same name and size")
)

// Skipping these only saves time: they hold dependencies, never test output.
var skippedDirNames = map[string]bool{
	".git":         true,
	".idea":        true,
	"node_modules": true,
	"Pods":         true,
	"Carthage":     true,
	"CordovaLib":   true,
	".build":       true,
	".dart_tool":   true,
	".venv":        true,
	"venv":         true,
}

// Candidate is an attachment file that belongs to a test case of the index.
type Candidate struct {
	Path  string
	Match Match
	info  fs.FileInfo
}

// Skipped is a file that follows the naming convention but is not attached, a folder that could
// not be read, or a file that could not be copied, with the reason.
type Skipped struct {
	Path   string
	Reason error
}

// CollectResult is the outcome of Collect.
type CollectResult struct {
	Candidates []Candidate
	Skipped    []Skipped
	// GitCheckErr is set when tracked files could not be filtered out (e.g. root is not in a git
	// repository). The candidates are then returned unfiltered.
	GitCheckErr error
}

// Collector finds and copies attachment files for a test report.
type Collector struct {
	cmdFactory  command.Factory
	fileManager fileutil.FileManager
}

// NewCollector returns a Collector that runs git through cmdFactory and copies files with fileManager.
func NewCollector(cmdFactory command.Factory, fileManager fileutil.FileManager) Collector {
	return Collector{cmdFactory: cmdFactory, fileManager: fileManager}
}

// Collect walks root and returns the files that belong to a test case of idx. Dependency folders,
// deployDir (where earlier steps exported their files), unreadable folders, files tracked by git,
// files an earlier step already exported unchanged and file names found more than once are left out.
// Of files with the same name and size, only the first by path is kept.
func (c Collector) Collect(root, deployDir string, idx Index) (CollectResult, error) {
	var result CollectResult
	var matched []Candidate
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			result.Skipped = append(result.Skipped, Skipped{Path: path, Reason: err})
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != root && (isSkippedDir(path) || filepath.Clean(path) == filepath.Clean(deployDir)) {
				return filepath.SkipDir
			}
			return nil
		}

		match, err := idx.Match(path)
		switch {
		case errors.Is(err, ErrUnsupportedType), errors.Is(err, ErrNoConvention):
			return nil
		case err != nil:
			result.Skipped = append(result.Skipped, Skipped{Path: path, Reason: err})
			return nil
		}
		info, err := os.Stat(path)
		if err != nil {
			result.Skipped = append(result.Skipped, Skipped{Path: path, Reason: err})
			return nil
		}
		matched = append(matched, Candidate{Path: path, Match: match, info: info})
		return nil
	})
	if err != nil {
		return CollectResult{}, fmt.Errorf("walk %s: %w", root, err)
	}

	// Duplicates are checked last: a committed reference image, or the file a previous run left
	// behind, usually has the same name as the fresh one and must not knock it out.
	untracked := matched
	tracked, err := c.trackedFiles(root, matched)
	if err != nil {
		result.GitCheckErr = err
	} else {
		untracked = nil
		for _, candidate := range matched {
			if tracked[candidate.Path] {
				result.Skipped = append(result.Skipped, Skipped{Path: candidate.Path, Reason: ErrTrackedByGit})
				continue
			}
			untracked = append(untracked, candidate)
		}
	}

	fresh, exported, err := skipExported(deployDir, untracked)
	if err != nil {
		return CollectResult{}, err
	}
	for _, candidate := range exported {
		result.Skipped = append(result.Skipped, Skipped{Path: candidate.Path, Reason: ErrAlreadyExported})
	}
	fresh, copies := withoutCopiesOfExported(fresh, exported)
	result.Skipped = append(result.Skipped, copies...)

	byName := map[string][]Candidate{}
	for _, candidate := range fresh {
		name := filepath.Base(candidate.Path)
		byName[name] = append(byName[name], candidate)
	}
	for _, name := range sortedKeys(byName) {
		found := byName[name]
		if len(found) == 1 {
			result.Candidates = append(result.Candidates, found[0])
			continue
		}
		if !sameSize(found) {
			for _, candidate := range found {
				result.Skipped = append(result.Skipped, Skipped{Path: candidate.Path, Reason: ErrDuplicateName})
			}
			continue
		}
		sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
		result.Candidates = append(result.Candidates, found[0])
		for _, candidate := range found[1:] {
			result.Skipped = append(result.Skipped, Skipped{Path: candidate.Path, Reason: ErrDuplicateCopy})
		}
	}
	return result, nil
}

func withoutCopiesOfExported(fresh, exported []Candidate) ([]Candidate, []Skipped) {
	exportedSizes := map[string]map[int64]bool{}
	for _, candidate := range exported {
		name := filepath.Base(candidate.Path)
		if exportedSizes[name] == nil {
			exportedSizes[name] = map[int64]bool{}
		}
		exportedSizes[name][candidate.info.Size()] = true
	}
	if len(exportedSizes) == 0 {
		return fresh, nil
	}

	var kept []Candidate
	var copies []Skipped
	for _, candidate := range fresh {
		if exportedSizes[filepath.Base(candidate.Path)][candidate.info.Size()] {
			copies = append(copies, Skipped{Path: candidate.Path, Reason: ErrDuplicateCopy})
			continue
		}
		kept = append(kept, candidate)
	}
	return kept, copies
}

func sameSize(candidates []Candidate) bool {
	for _, candidate := range candidates[1:] {
		if candidate.info.Size() != candidates[0].info.Size() {
			return false
		}
	}
	return true
}

func isSkippedDir(path string) bool {
	name := filepath.Base(path)
	if skippedDirNames[name] || strings.HasSuffix(name, ".framework") {
		return true
	}
	return name == "bundle" && filepath.Base(filepath.Dir(path)) == "vendor"
}

// gitBatchSize keeps each git ls-files call well below the OS limit on command line length.
const gitBatchSize = 500

// trackedFiles returns the candidate paths that git tracks.
func (c Collector) trackedFiles(root string, candidates []Candidate) (map[string]bool, error) {
	tracked := map[string]bool{}
	for start := 0; start < len(candidates); start += gitBatchSize {
		end := min(start+gitBatchSize, len(candidates))
		if err := c.addTrackedFiles(root, candidates[start:end], tracked); err != nil {
			return nil, err
		}
	}
	return tracked, nil
}

func (c Collector) addTrackedFiles(root string, candidates []Candidate, tracked map[string]bool) error {
	args := []string{"ls-files", "-z", "--"}
	relToPath := map[string]string{}
	for _, candidate := range candidates {
		rel, err := filepath.Rel(root, candidate.Path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		relToPath[rel] = candidate.Path
		args = append(args, rel)
	}

	out, err := c.cmdFactory.Create("git", args, &command.Opts{Dir: root}).RunAndReturnTrimmedOutput()
	if err != nil {
		return gitError(err)
	}

	for _, rel := range strings.Split(out, "\x00") {
		if path, ok := relToPath[rel]; ok {
			tracked[path] = true
		}
	}
	return nil
}

// gitError drops the command line from the error: with hundreds of paths it would flood the log.
func gitError(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("git ls-files: %s: %w", strings.TrimSpace(string(exitErr.Stderr)), exitErr)
	}
	if cause := errors.Unwrap(err); cause != nil {
		return fmt.Errorf("git ls-files: %w", cause)
	}
	return fmt.Errorf("git ls-files: %w", err)
}

// skipExported leaves out the candidates that an earlier step already exported into deployDir:
// a file with the same name, size and modification time. CopyToReport keeps the modification
// time, which is what makes an unchanged file recognisable.
func skipExported(deployDir string, candidates []Candidate) ([]Candidate, []Candidate, error) {
	exported := map[string][]fs.FileInfo{}
	err := filepath.WalkDir(deployDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == deployDir && errors.Is(err, fs.ErrNotExist) {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		exported[d.Name()] = append(exported[d.Name()], info)
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("walk %s: %w", deployDir, err)
	}

	var kept, skipped []Candidate
	for _, candidate := range candidates {
		if isExported(candidate.info, exported[filepath.Base(candidate.Path)]) {
			skipped = append(skipped, candidate)
			continue
		}
		kept = append(kept, candidate)
	}
	return kept, skipped, nil
}

func isExported(info fs.FileInfo, exported []fs.FileInfo) bool {
	for _, e := range exported {
		if e.Size() == info.Size() && e.ModTime().Equal(info.ModTime()) {
			return true
		}
	}
	return false
}

// CopyToReport copies the candidates into reportDir under their own name, keeping their
// modification time. A failed copy does not stop the others, it is returned in the result.
func (c Collector) CopyToReport(reportDir string, candidates []Candidate) []Skipped {
	var failed []Skipped
	for _, candidate := range candidates {
		dst := filepath.Join(reportDir, filepath.Base(candidate.Path))
		// Copying a file onto itself would truncate it. Possible when base_path is inside the deploy dir.
		if filepath.Clean(dst) == filepath.Clean(candidate.Path) {
			continue
		}
		if err := c.fileManager.CopyFile(candidate.Path, dst, &fileutil.CopyOptions{Overwrite: true}); err != nil {
			failed = append(failed, Skipped{Path: candidate.Path, Reason: fmt.Errorf("copy to %s: %w", reportDir, err)})
		}
	}
	return failed
}

func sortedKeys(m map[string][]Candidate) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
