package asset

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (s *SVGWalker) readAppIconSet() error {
	if s.Catalog.AppIcon != nil {
		return nil
	}

	appIcon, err := NewImageSet(filepath.Join(s.Dir, "AppIcon.appiconset"))
	if err != nil {
		return err
	}
	s.Catalog.AppIcon = appIcon
	return nil
}

func (s *SVGWalker) AddAppIconSVG(path string) error {
	if !strings.HasSuffix(path, ".svg") {
		return fmt.Errorf("%s: not an svg file", path)
	}
	if err := s.readAppIconSet(); err != nil {
		return err
	}
	if p, err := s.parseSVG(s.Catalog.AppIcon, path, 13); err != nil || !p.update {
		return err
	}
	name := filepath.Base(path)
	target := s.sanitized(name[0 : len(name)-4])
	makeImage := func(idiom string, scale int, size float32) Image {
		file := fmt.Sprintf("%s-%s-@%d-%d.png", target, idiom, scale, int(size))
		sizeStr := strings.TrimSuffix(fmt.Sprintf("%.1f", size), ".0")
		return Image{
			Scale:     fmt.Sprintf("%dx", scale),
			Size:      fmt.Sprintf("%sx%s", sizeStr, sizeStr),
			FileName:  file,
			Idiom:     idiom,
			generator: s.pngGenerator(s.Catalog.AppIcon, scale, size, size, path, file),
		}
	}
	s.Catalog.AppIcon.Images = []Image{
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
