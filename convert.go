package asset

import (
	"fmt"
	"os/exec"

	"encoding/base64"
	"html/template"
	"io/ioutil"
	"path/filepath"

	"github.com/urturn/go-phantomjs"
	"github.com/pkg/errors"
	"os"
)

type SVGConverter interface {
	Convert(scale, height, width int, svgFile, pngFile string) error
}

var ErrNoInkScape = errors.New("inkscape not installed. inkscape (https://www.inkscape.org/) is needed to convert SVG files.")

type InkScapeConverter struct{}

func (InkScapeConverter) Convert(scale, height, width int, svgFile, pngFile string) error {
	if _, err := exec.LookPath("inkscape"); err != nil {
		return ErrNoInkScape
	}
	cmd := exec.Command("inkscape",
		"--without-gui",
		"--export-height", fmt.Sprintf("%d", height*scale),
		"--export-width", fmt.Sprintf("%d", width*scale),
		"--export-png", pngFile,
		svgFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, string(out))
	}
	return nil
}

type PhantomJSConverter struct {
	p *phantomjs.Phantom
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
		done({'status': status, 'content': page.renderBase64('PNG')});
	});
};
`

func StartPhantomJSConverter() (*PhantomJSConverter, error) {
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
	return &PhantomJSConverter{p}, nil
}

func (p *PhantomJSConverter) Stop() error {
	return p.p.Exit()
}

func (p *PhantomJSConverter) Convert(scale, height, width int, svgFile, pngFile string) error {
	var result interface{}
	abs, err := filepath.Abs(svgFile)
	if err != nil {
		return errors.Wrap(err, "")
	}
	h, w := scale*height, scale*width
	call := fmt.Sprintf("function (done) {renderSVG(%q, %d, %d, done);}", "file://"+abs, h, w)
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
