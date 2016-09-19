package asset_test

import (
	"path/filepath"
	"testing"

	"io/ioutil"
	"os"

	"bytes"

	"github.com/sridharv/fail"
	"github.com/surullabs/indigo/tools/asset"
)

func TestCatalog(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	defer fail.Using(t.Fatal)

	tmpDir, err := ioutil.TempDir("", "catalog-test")
	fail.IfErr(err)
	defer os.RemoveAll(tmpDir)

	catalogDir := filepath.Join(tmpDir, "TestCatalog.xcassets")
	fail.IfErr(os.MkdirAll(catalogDir, 0700))
	catalog, err := asset.NewCatalog(catalogDir)
	fail.IfErr(err)

	// Test data sourced from https://github.com/encharm/Font-Awesome-SVG-PNG
	err = catalog.AddSVGs("testdata/data")
	if err == asset.ErrNoInkScape {
		t.Skip(err.Error())
	}
	fail.IfErr(err)
	fail.IfErr(catalog.Write())

	golden := listAll("testdata/TestCatalog.xcassets")
	actual := listAll(catalogDir)
	fail.If(len(golden) != len(actual), "expected ", len(golden), " files, got ", len(actual))
	for i, g := range golden {
		fail.If(g.name != actual[i].name, "expected ", g.name, ", got ", actual[i].name)
		fail.If(!bytes.Equal(g.contents, actual[i].contents), "contents of ", g.name, " not equal")
	}
}

type catalogFile struct {
	name     string
	contents []byte
}

func listAll(dir string) []catalogFile {
	files := []catalogFile{}
	fail.IfErr(filepath.Walk(dir, func(path string, stat os.FileInfo, err error) error {
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
