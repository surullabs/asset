package asset

import (
	"fmt"
	"os/exec"

	"encoding/base64"
	"html/template"
	"io/ioutil"
	"path/filepath"

	"os"

	"bytes"

	"strings"

	"encoding/xml"

	"github.com/pkg/errors"
	"github.com/urturn/go-phantomjs"
)

type SVGConverter interface {
	Convert(scale int, height, width float32, svgFile, pngFile string) error
}

var ErrNoInkScape = errors.New("inkscape not installed. inkscape (https://www.inkscape.org/) is needed to convert SVG files.")

type InkScapeConverter struct{}

func (InkScapeConverter) Convert(scale int, height, width float32, svgFile, pngFile string) error {
	if _, err := exec.LookPath("inkscape"); err != nil {
		return ErrNoInkScape
	}
	cmd := exec.Command("inkscape",
		"--without-gui",
		"--export-height", fmt.Sprintf("%f", int(float32(scale)*height)),
		"--export-width", fmt.Sprintf("%f", int(float32(scale)*width)),
		"--export-png", pngFile,
		svgFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, string(out))
	}
	return nil
}

type PhantomJSConverter struct {
	p   *phantomjs.Phantom
	dir string
}

var svgHTMLTemplate = template.Must(template.New("svg").Parse(`<!DOCTYPE html>
<style>
	html, body { margin: 0; padding: 0; }
	svg { position: absolute; top: 0; left: 0; }
</style>
<html>
	<body>
		<img src="{{.File}}" height="{{.Height}} width="{{.Width}}>
	</body>
</html>
`))

const converterJS = `
var page = require('webpage').create();

var renderSVG = function(source, height, width, done) {
	page.open(source, function(status) {
		if (status !== 'success') {
			done({'status': status});
			return;
		}
		page.viewportSize = {width: width, height: height};
        page.clipRect = {top: 0, left: 0, width: width, height: height};
        var content = page.renderBase64('PNG');
        page.stop();
        page.close();
		done({'status': status, 'content': content});
	});
};
`

func StartPhantomJSConverter() (*PhantomJSConverter, error) {
	tmpDir, err := ioutil.TempDir("", "phantomjs")
	if err != nil {
		return nil, err
	}
	p, err := phantomjs.Start()
	if err != nil {
		return nil, err
	}
	if err := p.Load(converterJS); err != nil {
		if exerr := p.Exit(); exerr != nil {
			return nil, fmt.Errorf("converter script error: %v (failed to exit: %v)", err, exerr)
		}
		return nil, err
	}
	return &PhantomJSConverter{p, tmpDir}, nil
}

func (p *PhantomJSConverter) Stop() error {
	defer os.RemoveAll(p.dir)
	return p.p.Exit()
}

func (p *PhantomJSConverter) Convert(scale int, height, width float32, svgFile, pngFile string) error {
	var result interface{}
	abs, err := filepath.Abs(svgFile)
	if err != nil {
		return errors.Wrap(err, "")
	}
	f := filepath.Join(p.dir, "out.html")
	var buf bytes.Buffer
	h, w := int(float32(scale)*height), int(float32(scale)*width)
	d := map[string]interface{}{"File": abs, "Height": h, "Width": w}
	if err = svgHTMLTemplate.Execute(&buf, d); err != nil {
		return errors.Wrapf(err, "%s: failed to generate html")
	}
	if err = ioutil.WriteFile(f, buf.Bytes(), 0600); err != nil {
		return errors.Wrapf(err, "%s: failed to write html")
	}
	call := fmt.Sprintf("function (done) {renderSVG(%q, %d, %d, done);}", "file://"+f, h, w)
	if err = p.p.Run(call, &result); err != nil {
		return errors.Wrapf(err, "%s: phantomjs convert failed", svgFile)
	}
	r, ok := result.(map[string]interface{})
	if !ok {
		return errors.Errorf("%s: expected map result, got: %T", svgFile, r)
	}
	if r["status"] != "success" {
		return errors.Errorf("%s: failed to render: %v", svgFile, r["status"])
	}
	data, err := base64.StdEncoding.DecodeString(r["content"].(string))
	if err != nil {
		return errors.Errorf("%s: failed to decode contents: %v", svgFile, r)
	}
	dir := filepath.Dir(pngFile)
	if _, err = os.Stat(dir); err != nil && os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0755); err != nil {
			return errors.Wrapf(err, "%s: failed to create dir", svgFile)
		}
	}
	if err := ioutil.WriteFile(pngFile, data, 0644); err != nil {
		return errors.Wrap(err, "failed to write")
	}
	return nil
}

type SVGWalker struct {
	Dir           string
	Converter     SVGConverter
	Catalog       *Catalog
	SanitizePaths bool
	ForceUpdate   bool
}

func (s *SVGWalker) sanitized(path string) string {
	if !s.SanitizePaths {
		return path
	}
	return strings.Replace(path, " ", "_", -1)
}

func (s *SVGWalker) Walk(path string, info os.FileInfo) error {
	if info.IsDir() || filepath.Ext(info.Name()) != ".svg" {
		return nil
	}
	f, err := filepath.Rel(s.Dir, path)
	if err != nil {
		return err
	}
	return s.add(f)
}

func (s *SVGWalker) add(file string) error {
	path, _ := filepath.Dir(file), filepath.Base(file)
	var holder Container = s.Catalog
	for path != "." && path != "" {
		var (
			group string
			err   error
		)
		path, group = filepath.Dir(path), filepath.Base(path)
		holder, err = holder.AddGroup(s.sanitized(group))
		if err != nil {
			return err
		}
	}
	return s.addSVG(holder, filepath.Join(s.Dir, file))
}

func (s *SVGWalker) addSVG(c Container, path string) error {
	if !strings.HasSuffix(path, ".svg") {
		return fmt.Errorf("%s: not an svg file", path)
	}

	name := filepath.Base(path)
	target := s.sanitized(name[0 : len(name)-4])

	image := c.ImageSet(target)
	if image == nil {
		var err error
		img := filepath.Join(c.Dir(), target+  ".imageset")
		if image, err = NewImageSet(img); err != nil {
			return err
		}
		c.SetImageSet(target, image)
	}
	p, err := s.parseSVG(image, path, 3)
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
			generator: s.pngGenerator(image, i, p.height, p.width, path, file),
		}
	}
	return nil
}

func (s *SVGWalker) pngGenerator(i *ImageSet, scale int, height, width float32, svg, out string) func() error {
	return func() error {
		file := filepath.Join(i.Dir, out)
		Log("Generating", file)
		return s.Converter.Convert(scale, height, width, svg, file)
	}
}

func (s *SVGWalker) parseSVG(i *ImageSet, path string, expected int) (parsedSVG, error) {
	update, err := s.needsUpdate(i, path, expected)
	if err != nil || !update {
		return parsedSVG{}, err
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return parsedSVG{}, err
	}
	var v svg
	if err := xml.Unmarshal(data, &v); err != nil {
		return parsedSVG{}, errors.Wrapf(err, "%s: failed to parse svg", path)
	}
	h, w, err := v.dim()
	if err != nil {
		return parsedSVG{}, errors.Wrapf(err, "%s: failed to parse dim", path)
	}
	return parsedSVG{update, h, w}, nil
}

func (s *SVGWalker) needsUpdate(i *ImageSet, svg string, expected int) (bool, error) {
	if s.ForceUpdate {
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
		stat, err := os.Stat(filepath.Join(i.Dir, image.FileName))
		if err != nil {
			return true, nil
		}
		if stat.ModTime().Before(svgStat.ModTime()) {
			return true, nil
		}
	}
	return false, nil
}

type parsedSVG struct {
	update bool
	height float32
	width  float32
}
