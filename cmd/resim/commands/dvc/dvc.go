// Package dvc translates local paths in a DVC repository into the
// dvc+s3://<bucket>/<remote prefix>//<repo-relative path>;<md5> experience
// locations understood by the ReSim platform. Translation is entirely local:
// hashes come from the repository's .dvc files (or the local DVC cache for
// files inside a tracked directory), and the bucket comes from the named
// remote in .dvc/config.
package dvc

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Locations with a URL scheme (s3://, dvc+s3://, https://, ...) are passed
// through untranslated; everything else is treated as a local path.
var schemeRegexp = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)

// The ReSim dvc+s3 location grammar separates the DVC remote's real S3 prefix
// from the repo-relative logical path with a double slash.
const remoteRootSeparator = "//"

// Directory hashes carry a ".dir" suffix in .dvc files and the DVC cache; the
// ReSim location grammar marks directories with a trailing slash instead.
const dirHashSuffix = ".dir"

// Resolver translates local paths into dvc+s3:// locations using a single
// named DVC remote. It caches per-repository config and directory manifests,
// so one Resolver can translate many paths cheaply.
type Resolver struct {
	remoteName string
	repos      map[string]*repository
}

func NewResolver(remoteName string) *Resolver {
	return &Resolver{
		remoteName: remoteName,
		repos:      map[string]*repository{},
	}
}

type repository struct {
	root      string
	remoteURL *url.URL
	// manifests caches parsed directory manifests by their (".dir"-suffixed) hash.
	manifests map[string][]manifestEntry
}

// A single entry of a DVC directory manifest (the JSON ".dir" object).
type manifestEntry struct {
	MD5     string `json:"md5"`
	Relpath string `json:"relpath"`
}

// TranslateLocation converts a local path into a dvc+s3:// location. Locations
// that already carry a URL scheme are returned unchanged.
func (r *Resolver) TranslateLocation(location string) (string, error) {
	if schemeRegexp.MatchString(location) {
		return location, nil
	}
	absPath, err := filepath.Abs(location)
	if err != nil {
		return "", fmt.Errorf("could not resolve path %q: %w", location, err)
	}
	repo, err := r.repositoryFor(absPath)
	if err != nil {
		return "", fmt.Errorf("could not translate %q: %w", location, err)
	}
	hash, err := repo.hashFor(absPath)
	if err != nil {
		return "", fmt.Errorf("could not translate %q: %w", location, err)
	}
	logicalPath, err := repo.logicalPath(absPath)
	if err != nil {
		return "", fmt.Errorf("could not translate %q: %w", location, err)
	}
	return formatLocation(repo.remoteURL, logicalPath, hash), nil
}

// repositoryFor finds the DVC repository containing path (by walking up
// towards the filesystem root looking for a .dvc directory) and loads its
// remote configuration, caching the result.
func (r *Resolver) repositoryFor(absPath string) (*repository, error) {
	root, err := findRepoRoot(absPath)
	if err != nil {
		return nil, err
	}
	if repo, ok := r.repos[root]; ok {
		return repo, nil
	}
	remoteURL, err := lookupRemoteURL(root, r.remoteName)
	if err != nil {
		return nil, err
	}
	repo := &repository{
		root:      root,
		remoteURL: remoteURL,
		manifests: map[string][]manifestEntry{},
	}
	r.repos[root] = repo
	return repo, nil
}

func findRepoRoot(absPath string) (string, error) {
	for dir := absPath; ; {
		if info, err := os.Stat(filepath.Join(dir, ".dvc")); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no DVC repository found containing %q (no .dvc directory in any parent)", absPath)
		}
		dir = parent
	}
}

// logicalPath returns the path relative to the repository root using forward
// slashes; this becomes the on-disk layout of the experience when it is run.
func (repo *repository) logicalPath(absPath string) (string, error) {
	rel, err := filepath.Rel(repo.root, absPath)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path is not inside the DVC repository %q", repo.root)
	}
	return filepath.ToSlash(rel), nil
}

// hashFor finds the DVC md5 hash for the given path. A path tracked directly
// (having a sibling "<path>.dvc" file) takes its hash from that file; a file
// inside a tracked directory is looked up in the directory's manifest from the
// local DVC cache. Directory hashes keep their ".dir" suffix.
func (repo *repository) hashFor(absPath string) (string, error) {
	if hash, err := hashFromDvcFile(absPath); err != nil {
		return "", err
	} else if hash != "" {
		return hash, nil
	}

	// Not tracked directly: look for a tracked ancestor directory.
	for dir := filepath.Dir(absPath); strings.HasPrefix(dir, repo.root) && dir != repo.root; dir = filepath.Dir(dir) {
		dirHash, err := hashFromDvcFile(dir)
		if err != nil {
			return "", err
		}
		if dirHash == "" {
			continue
		}
		if !strings.HasSuffix(dirHash, dirHashSuffix) {
			return "", fmt.Errorf("%q is tracked by DVC but its hash %q is not a directory hash", dir, dirHash)
		}
		return repo.hashFromManifest(dir, dirHash, absPath)
	}
	return "", fmt.Errorf("path is not tracked by DVC (no .dvc file found for it or any parent directory)")
}

// hashFromDvcFile returns the md5 hash recorded in "<path>.dvc", or "" if no
// such file exists.
func hashFromDvcFile(absPath string) (string, error) {
	dvcFilePath := absPath + ".dvc"
	data, err := os.ReadFile(dvcFilePath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("could not read %q: %w", dvcFilePath, err)
	}
	var dvcFile struct {
		Outs []struct {
			MD5  string `yaml:"md5"`
			Hash string `yaml:"hash"`
			Path string `yaml:"path"`
		} `yaml:"outs"`
	}
	if err := yaml.Unmarshal(data, &dvcFile); err != nil {
		return "", fmt.Errorf("could not parse %q: %w", dvcFilePath, err)
	}
	base := filepath.Base(absPath)
	for _, out := range dvcFile.Outs {
		if out.Path == base {
			return checkedMD5(out.MD5, out.Hash, dvcFilePath)
		}
	}
	if len(dvcFile.Outs) == 1 {
		return checkedMD5(dvcFile.Outs[0].MD5, dvcFile.Outs[0].Hash, dvcFilePath)
	}
	return "", fmt.Errorf("no output matching %q found in %q", base, dvcFilePath)
}

func checkedMD5(md5 string, hashKind string, dvcFilePath string) (string, error) {
	if md5 == "" {
		return "", fmt.Errorf("no md5 hash recorded in %q", dvcFilePath)
	}
	if hashKind != "" && hashKind != "md5" {
		return "", fmt.Errorf("unsupported hash type %q in %q: only md5 is supported", hashKind, dvcFilePath)
	}
	return md5, nil
}

// hashFromManifest resolves a file inside the tracked directory trackedDir by
// reading the directory's manifest from the local DVC cache.
func (repo *repository) hashFromManifest(trackedDir string, dirHash string, absPath string) (string, error) {
	entries, err := repo.loadManifest(dirHash)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(trackedDir, absPath)
	if err != nil {
		return "", err
	}
	relpath := filepath.ToSlash(rel)
	for _, entry := range entries {
		if entry.Relpath == relpath {
			return entry.MD5, nil
		}
		if strings.HasPrefix(entry.Relpath, relpath+"/") {
			return "", fmt.Errorf(
				"%q is a subdirectory of the DVC-tracked directory %q, which is not supported: reference the whole tracked directory, individual files, or track the subdirectory with dvc directly",
				absPath, trackedDir)
		}
	}
	return "", fmt.Errorf("%q not found in the DVC manifest of %q", relpath, trackedDir)
}

// loadManifest reads a directory manifest from the local DVC cache
// (.dvc/cache/files/md5/xx/yyy.dir, the DVC 3.x layout).
func (repo *repository) loadManifest(dirHash string) ([]manifestEntry, error) {
	if entries, ok := repo.manifests[dirHash]; ok {
		return entries, nil
	}
	if len(dirHash) <= 2 {
		return nil, fmt.Errorf("invalid DVC directory hash %q", dirHash)
	}
	cachePath := filepath.Join(repo.root, ".dvc", "cache", "files", "md5", dirHash[:2], dirHash[2:])
	data, err := os.ReadFile(cachePath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf(
			"DVC directory manifest %q is not in the local cache (%q): run `dvc fetch` first (note: only the DVC 3.x cache layout is supported)",
			dirHash, cachePath)
	}
	if err != nil {
		return nil, fmt.Errorf("could not read DVC directory manifest %q: %w", cachePath, err)
	}
	var entries []manifestEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("could not parse DVC directory manifest %q: %w", cachePath, err)
	}
	repo.manifests[dirHash] = entries
	return entries, nil
}

// formatLocation renders the dvc+s3 location string. A ".dir" hash suffix is
// converted to the trailing-slash directory marker.
func formatLocation(remoteURL *url.URL, logicalPath string, hash string) string {
	isDirectory := strings.HasSuffix(hash, dirHashSuffix)
	hash = strings.TrimSuffix(hash, dirHashSuffix)
	var sb strings.Builder
	sb.WriteString("dvc+s3://")
	sb.WriteString(remoteURL.Host)
	sb.WriteString("/")
	if prefix := strings.Trim(remoteURL.Path, "/"); prefix != "" {
		sb.WriteString(prefix)
		sb.WriteString(remoteRootSeparator)
	}
	sb.WriteString(logicalPath)
	sb.WriteString(";")
	sb.WriteString(hash)
	if isDirectory {
		sb.WriteString("/")
	}
	return sb.String()
}
