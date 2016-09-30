package asset

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

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
	*Container `json:"-"`
	AppIcon    *ImageSet   `json:"-"`
	Info       CatalogInfo `json:"info"`
}

func (c *Catalog) Write() error {
	if err := writeContents(c.Dir, c); err != nil {
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
	*Container `json:"-"`
	Info       CatalogInfo     `json:"info"`
	Properties GroupProperties `json:"properties"`
}

func (g *Group) Write() error {
	if err := os.MkdirAll(g.Dir, 0700); err != nil {
		return err
	}
	if err := writeContents(g.Dir, g); err != nil {
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

type Container struct {
	Dir    string
	Groups map[string]*Group
	Images map[string]*ImageSet
}

func NewContainer(dir string) *Container {
	return &Container{
		Dir:    dir,
		Groups: map[string]*Group{},
		Images: map[string]*ImageSet{},
	}
}

func (c *Container) AddGroup(name string) (*Group, error) {
	existing := c.Groups[name]
	if existing != nil {
		return existing, nil
	}
	group := &Group{
		Container: NewContainer(filepath.Join(c.Dir, name)),
	}
	exists, err := readContents(group.Dir, group)
	if err != nil {
		return nil, err
	}
	if !exists {
		group.Info = defaultCatalogInfo
		group.Properties = GroupProperties{ProvidesNamespace: true}
	}
	c.Groups[name] = group
	return group, nil
}

func (c *Container) write() error {
	for n, g := range c.Groups {
		if err := g.Write(); err != nil {
			return fmt.Errorf("%s:%v", n, err)
		}
	}

	for n, g := range c.Images {
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
	c := &Catalog{Container: NewContainer(dir)}
	if exists, err := readContents(dir, c); err != nil {
		return nil, err
	} else if !exists {
		c.Info = defaultCatalogInfo
	}
	return c, nil
}

