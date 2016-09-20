package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/surullabs/asset"
)

func validate(out string) error {
	if out == "" {
		return errors.New("no output directory specifed")
	}
	if len(flag.Args()) != 1 {
		return errors.New("no input directory specified")
	}
	if filepath.Ext(out) != ".xcassets" {
		return fmt.Errorf("unsupported output directory %s (must be end in .xcassets)", out)
	}
	return nil
}

func gen(out, appIcon string, force, sanitize bool) error {
	c, err := asset.NewCatalog(out, sanitize)
	if err != nil {
		return err
	}
	converter, err := asset.StartPhantomJSConverter()
	if err != nil {
		return err
	}
	defer func() {
		if err := converter.Stop(); err != nil {
			fmt.Fprintln(os.Stderr, "WARNING: failed to stop phantomjs cleanly: %v", err)
		}
	}()
	if err := c.AddSVGs(flag.Args()[0], force, converter); err != nil {
		return err
	}
	if appIcon != "" {
		if err := c.AddAppIconSVG(appIcon, force, converter); err != nil {
			return err
		}
	}
	return c.Write()
}

func main() {
	var (
		out, appIcon   string
		force, verbose, sanitize bool
	)
	flag.StringVar(&out, "out", "", "Output directory for the asset catalog")
	flag.StringVar(&appIcon, "appicon", "", "Path to the SVG to use as an app icon")
	flag.BoolVar(&force, "force", false, "If true all svgs are updated")
	flag.BoolVar(&verbose, "v", false, "If true verbose output is printed")
	flag.BoolVar(&sanitize, "sanitize", false, "If true any spaces in paths are converted into _")
	flag.Parse()

	if err := validate(out); err != nil {
		fmt.Fprintf(os.Stderr, "Usage: %s --out <path/to/Catalog.xcassets> <src>\n", os.Args[0])
		flag.Usage()
		os.Exit(1)
		return
	}

	if verbose {
		asset.Log = func(args ...interface{}) { fmt.Println(args...) }
	}

	err := gen(out, appIcon, force, sanitize)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
