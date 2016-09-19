package asset

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var Log = func(args ...interface{}) {}

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
	appIcon *ImageSet
	Info    CatalogInfo `json:"info"`
}

func (c *Catalog) Write() error {
	if err := writeContents(c.dir, c); err != nil {
		return err
	}
	if err := c.appIcon.Write(); err != nil {
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
	if i == nil {
		return nil
	}
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

func (i *ImageSet) needsUpdate(svg string, expected int, force bool) (bool, error) {
	if force {
		return true, nil
	}
	if len(i.Images) != expected {
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

func (i *ImageSet) parseSVG(path string, expected int, forceUpdate bool) (parsedSVG, error) {
	update, err := i.needsUpdate(path, expected, forceUpdate)
	if err != nil {
		return parsedSVG{}, err
	}
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return parsedSVG{}, err
	}
	var s svg
	if err := xml.Unmarshal(bytes, &s); err != nil {
		return parsedSVG{}, fmt.Errorf("%s: failed to parse svg:%v", path, err)
	}
	h, w, err := s.dim()
	if err != nil {
		return parsedSVG{}, fmt.Errorf("%s: failed to parse dim:%v", path, err)
	}
	return parsedSVG{update, h, w}, nil
}

type parsedSVG struct {
	update bool
	height float32
	width  float32
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
	p, err := image.parseSVG(path, 3, forceUpdate)
	if err != nil || !p.update {
		return err
	}

	image.Images = make([]Image, 3)
	for i := 1; i < 4; i++ {
		file := fmt.Sprintf("%s-%dx.png", target, i)
		image.Images[i-1] = Image{
			Scale:     fmt.Sprintf("%dx", i),
			FileName:  file,
			Idiom:     "universal",
			generator: image.pngGenerator(i, p.height, p.width, path, file, converter),
		}
	}
	return nil
}

func (i *ImageSet) pngGenerator(scale int, height, width float32, svg, out string, c SVGConverter) func() error {
	return func() error {
		file := filepath.Join(i.dir, out)
		Log("Generating", file)
		return c.Convert(scale, height, width, svg, file)
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

func (c *Catalog) readAppIconSet() error {
	if c.appIcon != nil {
		return nil
	}
	appIcon := &ImageSet{dir: filepath.Join(c.dir, "AppIcon.appiconset")}
	if exists, err := readContents(appIcon.dir, appIcon); err != nil {
		return err
	} else if !exists {
		appIcon.Info = defaultCatalogInfo
	}
	c.appIcon = appIcon
	return nil
}

func (c *Catalog) AddAppIconSVG(path string, force bool, converter SVGConverter) error {
	if !strings.HasSuffix(path, ".svg") {
		return fmt.Errorf("%s: not an svg file", path)
	}
	if err := c.readAppIconSet(); err != nil {
		return err
	}
	if p, err := c.appIcon.parseSVG(path, 13, force); err != nil || !p.update {
		return err
	}
	name := filepath.Base(path)
	target := name[0 : len(name)-4]
	makeImage := func(idiom string, scale int, size float32) Image {
		file := fmt.Sprintf("%s-%s-@%d-%d.png", target, idiom, scale, int(size))
		sizeStr := strings.TrimSuffix(fmt.Sprintf("%.1f", size), ".0")
		return Image{
			Scale:     fmt.Sprintf("%dx", scale),
			Size:      fmt.Sprintf("%sx%s", sizeStr, sizeStr),
			FileName:  file,
			Idiom:     idiom,
			generator: c.appIcon.pngGenerator(scale, size, size, path, file, converter),
		}
	}
	c.appIcon.Images = []Image{
		makeImage("iphone", 2, 29),
		makeImage("iphone", 3, 29),
		makeImage("iphone", 2, 40),
		makeImage("iphone", 3, 40),
		makeImage("iphone", 2, 60),
		makeImage("iphone", 3, 60),
		makeImage("ipad", 1, 29),
		makeImage("ipad", 2, 29),
		makeImage("ipad", 1, 40),
		makeImage("ipad", 2, 40),
		makeImage("ipad", 1, 76),
		makeImage("ipad", 2, 76),
		makeImage("ipad", 2, 83.5),
	}

	return nil
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
