package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslateDvcLocations(t *testing.T) {
	// A minimal DVC repository with one tracked file.
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".dvc"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".dvc", "config"), []byte(`
['remote "storage"']
    url = s3://my-bucket/dvcstore
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "file.txt.dvc"), []byte(`
outs:
- md5: 22a1a2931c8370d3aeedd7183606fd7f
  hash: md5
  path: file.txt
`), 0o644))

	localLocation := filepath.Join(root, "file.txt")
	config := &ExperienceSyncConfig{
		Experiences: []Experience{
			{Name: "one", Locations: []string{localLocation, "s3://other-bucket/foo"}},
		},
	}

	restore, err := translateDvcLocations(config, "storage")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"dvc+s3://my-bucket/dvcstore//file.txt;22a1a2931c8370d3aeedd7183606fd7f",
		"s3://other-bucket/foo",
	}, config.Experiences[0].Locations)

	// restore puts the original locations back so a config written to disk
	// keeps its local paths.
	restore()
	assert.Equal(t, []string{localLocation, "s3://other-bucket/foo"}, config.Experiences[0].Locations)
}

func TestTranslateDvcLocationsPropagatesErrors(t *testing.T) {
	config := &ExperienceSyncConfig{
		Experiences: []Experience{
			{Name: "one", Locations: []string{filepath.Join(t.TempDir(), "nowhere.txt")}},
		},
	}
	_, err := translateDvcLocations(config, "storage")
	require.ErrorContains(t, err, `experience "one"`)
}
