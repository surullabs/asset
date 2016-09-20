package asset

//go:generate mockgen -source=convert.go -destination=converter_mock_test.go -package=asset

import (
	"path/filepath"
	"testing"

	"io/ioutil"
	"os"

	"fmt"

	"github.com/JamesClonk/vultr/Godeps/_workspace/src/github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test data sourced from https://github.com/encharm/Font-Awesome-SVG-PNG

func TestConvert(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tmpDir, err := ioutil.TempDir("", "convert-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	calls := fakeCallsFromTestData(tmpDir)
	converter, err := StartPhantomJSConverter()
	require.NoError(t, err)
	defer func() { assert.NoError(t, converter.Stop()) }()
	for _, c := range calls {
		t.Run(c.src, func(t *testing.T) {
			require.NoError(t,
				converter.Convert(c.scale, c.height, c.width, c.svg, c.png),
				"failed to convert %+v", c.svg)
			golden, err := ioutil.ReadFile(c.src)
			require.NoError(t, err)
			actual, err := ioutil.ReadFile(c.png)
			require.NoError(t, err)
			require.Equal(t, golden, actual)
		})
		break
	}
}

type fakeConvertCall struct {
	scale  int
	height float32
	width  float32
	svg    string
	png    string
	src    string
}

type converter struct {
	calls  []fakeConvertCall
	called int
}

func (c *converter) Convert(scale int, height, width float32, svg, png string) error {
	c.called++
	idx := -1
	actual := fakeConvertCall{scale, height, width, svg, png, ""}
	for i, call := range c.calls {
		call.src = ""
		if actual == call {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("no matching calls found for %+v", actual)
	}
	call := c.calls[idx]
	b, err := ioutil.ReadFile(call.src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(png, b, 0600)
}

func fakeCall(tmpDir string, scale, height, width int, path string) fakeConvertCall {
	base := filepath.Base(path)
	file := filepath.Join(path+".imageset", fmt.Sprintf("%s-%dx.png", base, scale))
	svgFile := filepath.Join("testdata/data", path+".svg")
	pngFile := filepath.Join(tmpDir, "TestCatalog.xcassets", file)
	golden := filepath.Join("testdata/TestCatalog.xcassets", file)
	return fakeConvertCall{scale, float32(height), float32(width), svgFile, pngFile, golden}
}

func fakeCallsFromTestData(tmpDir string) []fakeConvertCall {
	return []fakeConvertCall{
		fakeCall(tmpDir, 1, 150, 150, "folder1/home"),
		fakeCall(tmpDir, 2, 150, 150, "folder1/home"),
		fakeCall(tmpDir, 3, 150, 150, "folder1/home"),
		fakeCall(tmpDir, 1, 150, 150, "info"),
		fakeCall(tmpDir, 2, 150, 150, "info"),
		fakeCall(tmpDir, 3, 150, 150, "info"),
		fakeCall(tmpDir, 1, 150, 150, "lock"),
		fakeCall(tmpDir, 2, 150, 150, "lock"),
		fakeCall(tmpDir, 3, 150, 150, "lock"),
	}
}

func TestCatalog(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "catalog-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	catalogDir := filepath.Join(tmpDir, "TestCatalog.xcassets")
	require.NoError(t, os.MkdirAll(catalogDir, 0700))
	catalog, err := NewCatalog(catalogDir, false)
	require.NoError(t, err)

	mock := &converter{calls: fakeCallsFromTestData(tmpDir)}
	require.NoError(t, catalog.AddSVGs("testdata/data", false, mock))
	require.NoError(t, catalog.Write())
	require.Equal(t, len(mock.calls), mock.called)

	golden := listAll(t, "testdata/TestCatalog.xcassets")
	actual := listAll(t, catalogDir)
	require.Len(t, actual, len(golden), "expected ", len(golden), " files, got ", len(actual))
	for i, g := range golden {
		require.Equal(t, g.name, actual[i].name, "expected ", g.name, ", got ", actual[i].name)
		require.Equal(t, g.contents, actual[i].contents, "contents of ", g.name, " not equal")
	}

	mock = &converter{calls: nil}
	require.NoError(t, catalog.AddSVGs("testdata/data", false, mock))
	require.NoError(t, catalog.Write())
	require.Equal(t, 0, mock.called)

	// Now force an update
	mock = &converter{calls: fakeCallsFromTestData(tmpDir)}
	require.NoError(t, catalog.AddSVGs("testdata/data", true, mock))
	require.NoError(t, catalog.Write())
	require.Equal(t, len(mock.calls), mock.called)

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
