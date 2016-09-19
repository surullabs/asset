package asset

import (
	"errors"
	"fmt"
	"os/exec"
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
