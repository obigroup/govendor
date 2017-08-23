// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package migrate

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/obigroup/govendor/context"
	"github.com/obigroup/govendor/vendorfile"
	"gopkg.in/yaml.v2"
)

func init() {
	register("glide", sysGlide{})
}

type sysGlide struct{}

func (sys sysGlide) Check(root string) (system, error) {
	// Glide has two config files: glide.yaml and glide.lock. The
	// first file is for manual configuration. The second file is
	// autogenerated from the first one.  Migration procedure uses
	// autogenerated glide.lock because it has resolved recursive
	// dependencies that glide make automatically from glide.yaml.
	if hasFiles(root, "glide.lock") {
		return sys, nil
	}
	return nil, nil
}

func (sys sysGlide) Migrate(root string) error {
	// Create a new empty config.
	ctx, err := context.NewContext(root, filepath.Join("vendor", "vendor.json"), "vendor", false)
	if err != nil {
		return err
	}
	ctx.VendorDiscoverFolder = "vendor"
	ctx.VendorFile.Ignore = "test"

	// Get&parse glide' config.
	rawConfigData, err := ioutil.ReadFile(filepath.Join(root, "glide.lock"))
	if err != nil {
		return err
	}
	type (
		imports struct {
			Name        string   `json:"name"`
			Version     string   `json:"version"`
			Repo        string   `json:"repo,omitempty"`
			Subpackages []string `json:"subpackages,omitempty"`
		}
		glideLock struct {
			Imports []imports `json:"imports"`
		}
	)
	parsedConfig := glideLock{}
	err = yaml.Unmarshal(rawConfigData, &parsedConfig)
	if err != nil {
		return err
	}

	// Build a new config.
	for _, i := range parsedConfig.Imports {
		pkg := vendorfile.Package{
			Add:      true,
			Path:     i.Name,
			Revision: i.Version,
		}
		if len(i.Subpackages) > 0 {
			for _, p := range i.Subpackages {
				subpkg := vendorfile.Package{
					Add:      true,
					Path:     path.Join(i.Name, p),
					Revision: i.Version,
				}
				if i.Repo != "" {
					subpkg.Origin = path.Join(i.Repo, p)
				}
				ctx.VendorFile.Package = append(ctx.VendorFile.Package, &subpkg)
			}
		}
		if i.Repo != "" {
			pkg.Origin = i.Repo
		}
		ctx.VendorFile.Package = append(ctx.VendorFile.Package, &pkg)
	}
	err = ctx.WriteVendorFile()
	if err != nil {
		return err
	}

	// Cleanup.
	os.RemoveAll(filepath.Join(root, "glide.yaml"))
	return os.RemoveAll(filepath.Join(root, "glide.lock"))
}
