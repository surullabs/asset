package asset

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

var Log = func(args ...interface{}) {}

func readContents(dir string, v interface{}) (bool, error) {
	contents, err := os.Open(filepath.Join(dir, "Contents.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "failed to open Contents.json")
	}
	if err := json.NewDecoder(contents).Decode(v); err != nil {
		return false, errors.Wrapf(err, "failed to decode %s/Contents.json", dir)
	}
	return true, nil
}

func writeContents(dir string, v interface{}) error {
	contents, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(dir, "Contents.json"), contents, 0600)
}

type Catalog struct {
	*container
	AppIcon *ImageSet   `json:"-"`
	Info    CatalogInfo `json:"info"`
}

func (c *Catalog) Write() error {
	if err := writeContents(c.dir, c); err != nil {
		return err
	}
	if err := c.AppIcon.Write(); err != nil {
		return err
	}
	return c.write()
}

var defaultCatalogInfo = CatalogInfo{
	Author:  "indigo",
	Version: 1,
}

type CatalogInfo struct {
	Author  string `json:"author"`
	Version int    `json:"version"`
}

type Group struct {
	*container
	Info       CatalogInfo     `json:"info"`
	Properties GroupProperties `json:"properties"`
}

func (g *Group) Write() error {
	if err := os.MkdirAll(g.dir, 0700); err != nil {
		return err
	}
	if err := writeContents(g.dir, g); err != nil {
		return err
	}
	return g.write()
}

type GroupProperties struct {
	ResourceTags
	ProvidesNamespace bool `json:"provides-namespace"`
}

type ResourceTags struct {
	OnDemandResourceTags []string `json:"on-demand-resource-tags,omitempty"`
}

type ImageSet struct {
	Dir        string      `json:"-"`
	Info       CatalogInfo `json:"info"`
	Properties ResourceTags
	Images     []Image `json:"images"`
}

func NewImageSet(path string) (*ImageSet, error) {
	image := &ImageSet{
		Dir: filepath.Join(path),
	}
	exists, err := readContents(image.Dir, image)
	if err != nil {
		return nil, err
	}
	if !exists {
		image.Info = defaultCatalogInfo
	}
	return image, nil
}

func (i *ImageSet) Write() error {
	if i == nil {
		return nil
	}
	if err := os.MkdirAll(i.Dir, 0700); err != nil {
		return err
	}
	for j, image := range i.Images {
		if image.generator == nil {
			continue
		}
		i.Images[j].generator = nil
		if err := image.generator(); err != nil {
			return err
		}
	}
	return writeContents(i.Dir, i)
}

type Image struct {
	FileName           string                 `json:"filename"`
	Size               string                 `json:"size,omitempty"`
	GraphicsFeatureSet string                 `json:"graphics-feature-set,omitempty"`
	Idiom              string                 `json:"idiom,omitempty"`
	Memory             string                 `json:"memory,omitempty"`
	Scale              string                 `json:"scale,omitempty"`
	Subtype            string                 `json:"subtype,omitempty"`
	ScreenWidth        string                 `json:"screen-width,omitempty"`
	WidthClass         string                 `json:"width-class,omitempty"`
	HeightClass        string                 `json:"height-class,omitempty"`
	Unassigned         bool                   `json:"unassigned,omitempty"`
	AlignmentInsets    map[string]interface{} `json:"alignment-insets,omitempty"`
	generator          func() error
}

type container struct {
	dir    string
	name   string
	g      map[string]*Group
	images map[string]*ImageSet
}

func (c *container) Dir() string                          { return c.dir }
func (c *container) SetImageSet(name string, i *ImageSet) { c.images[name] = i }
func (c *container) ImageSet(name string) *ImageSet       { return c.images[name] }

func newContainer(name, dir string) *container {
	return &container{
		dir:    dir,
		name:   name,
		g:      map[string]*Group{},
		images: map[string]*ImageSet{},
	}
}

type svg struct {
	Height string `xml:"height,attr"`
	Width  string `xml:"width,attr"`
}

func (s svg) dim() (float32, float32, error) {
	h, err := parseDim(s.Height)
	if err != nil {
		return 0, 0, err
	}
	w, err := parseDim(s.Width)
	if err != nil {
		return 0, 0, err
	}
	return h, w, nil
}

func parseDim(str string) (v float32, err error) {
	defer func() {
		if v == 0 {
			v = 150
		}
	}()
	switch {
	case str == "":
		return 0, nil
	case strings.HasSuffix(str, "px"):
		str = strings.TrimSuffix(str, "px")
	}
	val, err := strconv.ParseFloat(str, 32)
	return float32(val), err
}

func (c *container) AddGroup(name string) (Container, error) {
	existing := c.g[name]
	if existing != nil {
		return existing, nil
	}
	group := &Group{
		container: newContainer(name, filepath.Join(c.dir, name)),
	}
	exists, err := readContents(group.dir, group)
	if err != nil {
		return nil, err
	}
	if !exists {
		group.Info = defaultCatalogInfo
		group.Properties = GroupProperties{ProvidesNamespace: true}
	}
	c.g[name] = group
	return group, nil
}

func (c *container) write() error {
	for n, g := range c.g {
		if err := g.Write(); err != nil {
			return fmt.Errorf("%s:%v", n, err)
		}
	}

	for n, g := range c.images {
		if err := g.Write(); err != nil {
			return fmt.Errorf("%s:%v", n, err)
		}
	}
	return nil
}

func NewCatalog(dir string) (*Catalog, error) {
	fileName := filepath.Base(dir)
	ext := filepath.Ext(fileName)
	if ext != ".xcassets" {
		return nil, fmt.Errorf("%s:not a catalog folder", dir)
	}
	stat, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("%s: not a directory", dir)
	}
	c := &Catalog{container: newContainer(strings.TrimSuffix(fileName, ext), dir)}
	if exists, err := readContents(dir, c); err != nil {
		return nil, err
	} else if !exists {
		c.Info = defaultCatalogInfo
	}
	return c, nil
}

type Walker interface {
	Walk(path string, info os.FileInfo) error
}

func (c *Catalog) Walk(dir string, walker Walker) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return walker.Walk(path, info)
	})
}

type Container interface {
	Dir() string
	AddGroup(name string) (Container, error)
	ImageSet(name string) *ImageSet
	SetImageSet(name string, i *ImageSet)
	Write() error
}
