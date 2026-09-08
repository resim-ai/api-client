package dvc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fileHash   = "22a1a2931c8370d3aeedd7183606fd7f"
	dirHash    = "aabbccddeeff00112233445566778899"
	scene1Hash = "11111111111111111111111111111111"
	scene2Hash = "22222222222222222222222222222222"
)

// newTestRepo lays out a minimal DVC repository:
//
//	repo/
//	  .dvc/config                # remotes: storage (s3 w/ prefix), rootremote (bucket root), gcs (non-s3)
//	  .dvc/cache/files/md5/...   # the directory manifest for scenarios/
//	  data/file.txt.dvc          # tracked file
//	  scenarios.dvc              # tracked directory: scene1.bag, nested/scene2.bag
func newTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	writeFile(t, filepath.Join(root, ".dvc", "config"), `
[core]
    remote = storage
['remote "storage"']
    url = s3://my-bucket/dvcstore
['remote "rootremote"']
    url = s3://root-bucket
['remote "gcs"']
    url = gs://not-s3/path
`)

	writeFile(t, filepath.Join(root, "data", "file.txt.dvc"), `
outs:
- md5: `+fileHash+`
  size: 14445097
  hash: md5
  path: file.txt
`)

	writeFile(t, filepath.Join(root, "scenarios.dvc"), `
outs:
- md5: `+dirHash+`.dir
  size: 1200
  nfiles: 2
  hash: md5
  path: scenarios
`)

	writeFile(t, filepath.Join(root, ".dvc", "cache", "files", "md5", dirHash[:2], dirHash[2:]+".dir"),
		`[{"md5": "`+scene1Hash+`", "relpath": "scene1.bag"},
		  {"md5": "`+scene2Hash+`", "relpath": "nested/scene2.bag"}]`)

	return root
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestPassesThroughRemoteLocations(t *testing.T) {
	resolver := NewResolver("storage")
	for _, location := range []string{
		"s3://my-bucket/foo",
		"dvc+s3://my-bucket/foo;abc123",
		"https://example.com/foo",
	} {
		translated, err := resolver.TranslateLocation(location)
		require.NoError(t, err)
		assert.Equal(t, location, translated)
	}
}

func TestTranslatesTrackedFile(t *testing.T) {
	root := newTestRepo(t)
	resolver := NewResolver("storage")

	translated, err := resolver.TranslateLocation(filepath.Join(root, "data", "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "dvc+s3://my-bucket/dvcstore//data/file.txt;"+fileHash, translated)
}

func TestTranslatesWithBucketRootRemote(t *testing.T) {
	root := newTestRepo(t)
	resolver := NewResolver("rootremote")

	translated, err := resolver.TranslateLocation(filepath.Join(root, "data", "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "dvc+s3://root-bucket/data/file.txt;"+fileHash, translated)
}

func TestTranslatesTrackedDirectory(t *testing.T) {
	root := newTestRepo(t)
	resolver := NewResolver("storage")

	translated, err := resolver.TranslateLocation(filepath.Join(root, "scenarios"))
	require.NoError(t, err)
	assert.Equal(t, "dvc+s3://my-bucket/dvcstore//scenarios;"+dirHash+"/", translated)
}

func TestTranslatesFileInsideTrackedDirectory(t *testing.T) {
	root := newTestRepo(t)
	resolver := NewResolver("storage")

	translated, err := resolver.TranslateLocation(filepath.Join(root, "scenarios", "scene1.bag"))
	require.NoError(t, err)
	assert.Equal(t, "dvc+s3://my-bucket/dvcstore//scenarios/scene1.bag;"+scene1Hash, translated)

	translated, err = resolver.TranslateLocation(filepath.Join(root, "scenarios", "nested", "scene2.bag"))
	require.NoError(t, err)
	assert.Equal(t, "dvc+s3://my-bucket/dvcstore//scenarios/nested/scene2.bag;"+scene2Hash, translated)
}

func TestRejectsSubdirectoryOfTrackedDirectory(t *testing.T) {
	root := newTestRepo(t)
	resolver := NewResolver("storage")

	_, err := resolver.TranslateLocation(filepath.Join(root, "scenarios", "nested"))
	require.ErrorContains(t, err, "subdirectory")
}

func TestRejectsFileMissingFromManifest(t *testing.T) {
	root := newTestRepo(t)
	resolver := NewResolver("storage")

	_, err := resolver.TranslateLocation(filepath.Join(root, "scenarios", "missing.bag"))
	require.ErrorContains(t, err, "not found in the DVC manifest")
}

func TestRejectsUntrackedPath(t *testing.T) {
	root := newTestRepo(t)
	resolver := NewResolver("storage")

	_, err := resolver.TranslateLocation(filepath.Join(root, "untracked.txt"))
	require.ErrorContains(t, err, "not tracked by DVC")
}

func TestRejectsPathOutsideRepo(t *testing.T) {
	outside := t.TempDir()
	resolver := NewResolver("storage")

	_, err := resolver.TranslateLocation(filepath.Join(outside, "file.txt"))
	require.ErrorContains(t, err, "no DVC repository found")
}

func TestRejectsUnknownRemote(t *testing.T) {
	root := newTestRepo(t)
	resolver := NewResolver("nonexistent")

	_, err := resolver.TranslateLocation(filepath.Join(root, "data", "file.txt"))
	require.ErrorContains(t, err, `remote "nonexistent" not found`)
}

func TestRejectsNonS3Remote(t *testing.T) {
	root := newTestRepo(t)
	resolver := NewResolver("gcs")

	_, err := resolver.TranslateLocation(filepath.Join(root, "data", "file.txt"))
	require.ErrorContains(t, err, "only s3:// remotes are supported")
}

func TestRejectsMissingCacheManifest(t *testing.T) {
	root := newTestRepo(t)
	require.NoError(t, os.RemoveAll(filepath.Join(root, ".dvc", "cache")))
	resolver := NewResolver("storage")

	_, err := resolver.TranslateLocation(filepath.Join(root, "scenarios", "scene1.bag"))
	require.ErrorContains(t, err, "dvc fetch")
}

func TestConfigLocalOverridesConfig(t *testing.T) {
	root := newTestRepo(t)
	writeFile(t, filepath.Join(root, ".dvc", "config.local"), `
['remote "storage"']
    url = s3://local-bucket/other
`)
	resolver := NewResolver("storage")

	translated, err := resolver.TranslateLocation(filepath.Join(root, "data", "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "dvc+s3://local-bucket/other//data/file.txt;"+fileHash, translated)
}

func TestTranslatesRelativePath(t *testing.T) {
	root := newTestRepo(t)
	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	resolver := NewResolver("storage")
	translated, err := resolver.TranslateLocation(filepath.Join("data", "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "dvc+s3://my-bucket/dvcstore//data/file.txt;"+fileHash, translated)
}
