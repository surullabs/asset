package asset

import (
	"path/filepath"
	"testing"

	"io/ioutil"
	"os"

	"github.com/stretchr/testify/require"
)

func TestCatalog(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tmpDir, err := ioutil.TempDir("", "catalog-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	catalogDir := filepath.Join(tmpDir, "TestCatalog.xcassets")
	require.NoError(t, os.MkdirAll(catalogDir, 0700))
	catalog, err := NewCatalog(catalogDir)
	require.NoError(t, err)

	// Test data sourced from https://github.com/encharm/Font-Awesome-SVG-PNG
	err = catalog.AddSVGs("testdata/data")
	if err == ErrNoInkScape {
		t.Skip(err.Error())
	}
	require.NoError(t, err)
	require.NoError(t, catalog.Write())

	golden := listAll(t, "testdata/TestCatalog.xcassets")
	actual := listAll(t, catalogDir)
	require.Equal(t, len(golden), len(actual), "expected ", len(golden), " files, got ", len(actual))
	for i, g := range golden {
		require.Equal(t, g.name, actual[i].name, "expected ", g.name, ", got ", actual[i].name)
		require.Equal(t, g.contents, actual[i].contents, "contents of ", g.name, " not equal")
	}
}

type catalogFile struct {
	name     string
	contents []byte
}

func listAll(t *testing.T, dir string) []catalogFile {
	files := []catalogFile{}
	require.NoError(t, filepath.Walk(dir, func(path string, stat os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		c := catalogFile{}
		if c.name, err = filepath.Rel(dir, path); err != nil {
			return err
		}
		if !stat.IsDir() {
			if c.contents, err = ioutil.ReadFile(path); err != nil {
				return err
			}
		}
		files = append(files, c)
		return nil
	}))
	return files
}
