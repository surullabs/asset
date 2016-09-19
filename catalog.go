package asset

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"strconv"
)

func readContents(dir string, v interface{}) (bool, error) {
	contents, err := os.Open(filepath.Join(dir, "Contents.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if err := json.NewDecoder(contents).Decode(v); err != nil {
		return false, err
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
	Info CatalogInfo `json:"info"`
}

func (c *Catalog) Write() error {
	if err := writeContents(c.dir, c); err != nil {
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
	dir        string
	Info       CatalogInfo `json:"info"`
	Properties ResourceTags
	Images     []Image `json:"images"`
}

func (i *ImageSet) Write() error {
	if err := os.MkdirAll(i.dir, 0700); err != nil {
		return err
	}
	for _, image := range i.Images {
		if image.generator == nil {
			continue
		}
		if err := image.generator(); err != nil {
			return err
		}
	}
	return writeContents(i.dir, i)
}

type Image struct {
	FileName           string                 `json:"filename"`
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

func (s svg) dim() (int, int, error) {
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

func parseDim(str string) (v int, err error) {
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
	val, err := strconv.ParseInt(str, 10, 32)
	return int(val), err
}

func (i *ImageSet) needsUpdate(svg string) (bool, error) {
	if len(i.Images) != 3 {
		return true, nil
	}
	svgStat, err := os.Stat(svg)
	if err != nil {
		return false, err
	}
	for _, image := range i.Images {
		stat, err := os.Stat(filepath.Join(i.dir, image.FileName))
		if err != nil {
			return true, nil
		}
		if stat.ModTime().Before(svgStat.ModTime()) {
			return true, nil
		}
	}
	return false, nil
}

func (c *container) AddSVG(path string, forceUpdate bool, converter SVGConverter) error {
	if !strings.HasSuffix(path, ".svg") {
		return fmt.Errorf("%s: not an svg file", path)
	}

	name := filepath.Base(path)
	target := name[0 : len(name)-4]

	image := c.images[target]
	if image == nil {
		image = &ImageSet{
			dir: filepath.Join(c.dir, target+".imageset"),
		}
		exists, err := readContents(image.dir, image)
		if err != nil {
			return err
		}
		if !exists {
			image.Info = defaultCatalogInfo
		}
		c.images[target] = image
	}

	if !forceUpdate {
		if update, err := image.needsUpdate(path); !update {
			return err
		}
	}

	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	var s svg
	if err := xml.Unmarshal(bytes, &s); err != nil {
		return fmt.Errorf("%s: failed to parse svg:%v", path, err)
	}
	h, w, err := s.dim()
	if err != nil {
		return fmt.Errorf("%s: failed to parse dim:%v", path, err)
	}

	image.Images = make([]Image, 3)
	for i := 1; i < 4; i++ {
		image.Images[i-1] = Image{
			Scale:     fmt.Sprintf("%dx", i),
			FileName:  fmt.Sprintf("%s-%dx.png", target, i),
			Idiom:     "universal",
			generator: image.pngGenerator(i, h, w, path, converter),
		}
	}
	return nil
}

func (i *ImageSet) pngGenerator(scale, height, width int, svg string, c SVGConverter) func() error {
	return func() error {
		return c.Convert(scale, height, width, svg, filepath.Join(i.dir, i.Images[scale-1].FileName))
	}
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

// Go through a folder and convert all SVG files to PDF files for iOS

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

func (c *Catalog) AddSVGs(dir string, forceUpdate bool, converter SVGConverter) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || filepath.Ext(info.Name()) != ".svg" {
			return nil
		}
		f, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		return c.addSVG(dir, f, forceUpdate, converter)
	})
}

func (c *Catalog) addSVG(dir, file string, forceUpdate bool, converter SVGConverter) error {
	path, _ := filepath.Dir(file), filepath.Base(file)
	var holder Container = c
	for path != "." && path != "" {
		var (
			group string
			err   error
		)
		path, group = filepath.Dir(path), filepath.Base(path)
		holder, err = holder.AddGroup(group)
		if err != nil {
			return err
		}
	}
	return holder.AddSVG(filepath.Join(dir, file), forceUpdate, converter)
}

type Container interface {
	AddGroup(name string) (Container, error)
	AddSVG(file string, forceUpdate bool, c SVGConverter) error
	Write() error
}
