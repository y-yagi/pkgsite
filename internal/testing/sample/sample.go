// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sample provides functionality for generating sample values of
// the types contained in the internal package.
package sample

import (
	"fmt"
	"math"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/licensecheck"
	"github.com/google/safehtml/template"
	"golang.org/x/pkgsite/internal"
	"golang.org/x/pkgsite/internal/licenses"
	"golang.org/x/pkgsite/internal/source"
	"golang.org/x/pkgsite/internal/stdlib"
)

// These sample values can be used to construct test cases.
var (
	ModulePath      = "github.com/valid/module_name"
	RepositoryURL   = "https://github.com/valid/module_name"
	VersionString   = "v1.0.0"
	CommitTime      = NowTruncated()
	LicenseMetadata = []*licenses.Metadata{
		{
			Types:    []string{"MIT"},
			FilePath: "LICENSE",
			Coverage: licensecheck.Coverage{
				Percent: 100,
				Match:   []licensecheck.Match{{Name: "MIT", Type: licensecheck.MIT, Percent: 100}},
			},
		},
	}
	Licenses = []*licenses.License{
		{Metadata: LicenseMetadata[0], Contents: []byte(`Lorem Ipsum`)},
	}
	NonRedistributableLicense = &licenses.License{
		Metadata: &licenses.Metadata{
			FilePath: "NONREDIST_LICENSE",
			Types:    []string{"UNKNOWN"},
		},
		Contents: []byte(`unknown`),
	}
	DocumentationHTML = template.MustParseAndExecuteToHTML("This is the documentation HTML")
	PackageName       = "foo"
	Suffix            = "foo"
	PackagePath       = path.Join(ModulePath, Suffix)
	V1Path            = PackagePath
	Imports           = []string{"path/to/bar", "fmt"}
	Synopsis          = "This is a package synopsis"
	ReadmeFilePath    = "README.md"
	ReadmeContents    = "readme"
	GOOS              = "linux"
	GOARCH            = "amd64"
	Documentation     = &internal.Documentation{
		Synopsis: Synopsis,
		HTML:     DocumentationHTML,
		GOOS:     GOOS,
		GOARCH:   GOARCH,
	}
)

// LicenseCmpOpts are options to use when comparing licenses with the cmp package.
var LicenseCmpOpts = []cmp.Option{
	cmp.Comparer(coveragePercentEqual),
	cmpopts.IgnoreFields(licensecheck.Match{}, "Start", "End"),
}

// coveragePercentEqual considers two floats the same if they are within 4
// percentage points, and both are on the same side of 90% (our threshold).
func coveragePercentEqual(a, b float64) bool {
	if (a >= 90) != (b >= 90) {
		return false
	}
	return math.Abs(a-b) <= 4
}

// NowTruncated returns time.Now() truncated to Microsecond precision.
//
// This makes it easier to work with timestamps in PostgreSQL, which have
// Microsecond precision:
//   https://www.postgresql.org/docs/9.1/datatype-datetime.html
func NowTruncated() time.Time {
	return time.Now().Truncate(time.Microsecond)
}

// LegacyPackage constructs a package with the given module path and suffix.
//
// If modulePath is the standard library, the package path is the
// suffix, which must not be empty. Otherwise, the package path
// is the concatenation of modulePath and suffix.
//
// The package name is last component of the package path.
func LegacyPackage(modulePath, suffix string) *internal.LegacyPackage {
	p := constructFullPath(modulePath, suffix)
	return &internal.LegacyPackage{
		Name:              path.Base(p),
		Path:              p,
		V1Path:            internal.V1Path(p, modulePath),
		Synopsis:          Synopsis,
		IsRedistributable: true,
		Licenses:          LicenseMetadata,
		DocumentationHTML: DocumentationHTML,
		Imports:           Imports,
		GOOS:              GOOS,
		GOARCH:            GOARCH,
	}
}

func PackageMeta(fullPath string) *internal.PackageMeta {
	return &internal.PackageMeta{
		Path:              fullPath,
		IsRedistributable: true,
		Name:              path.Base(fullPath),
		Synopsis:          Synopsis,
		Licenses:          LicenseMetadata,
	}
}

func LegacyModuleInfo(modulePath, versionString string) *internal.LegacyModuleInfo {
	mi := ModuleInfoReleaseType(modulePath, versionString)
	return &internal.LegacyModuleInfo{
		ModuleInfo:           *mi,
		LegacyReadmeFilePath: ReadmeFilePath,
		LegacyReadmeContents: ReadmeContents,
	}
}

func ModuleInfo(modulePath, versionString string) *internal.ModuleInfo {
	mi := ModuleInfoReleaseType(modulePath, versionString)
	return mi
}

// We shouldn't need this, but some code (notably frontend/directory_test.go) creates
// ModuleInfos with "latest" for version, which should not be valid.
func ModuleInfoReleaseType(modulePath, versionString string) *internal.ModuleInfo {
	return &internal.ModuleInfo{
		ModulePath: modulePath,
		Version:    versionString,
		CommitTime: CommitTime,
		// Assume the module path is a GitHub-like repo name.
		SourceInfo:        source.NewGitHubInfo("https://"+modulePath, "", versionString),
		IsRedistributable: true,
		HasGoMod:          true,
	}
}

func DefaultModule() *internal.Module {
	return AddPackage(
		Module(ModulePath, VersionString),
		LegacyPackage(ModulePath, Suffix))
}

func DefaultVersionMap() *internal.VersionMap {
	return &internal.VersionMap{
		ModulePath:       ModulePath,
		RequestedVersion: VersionString,
		ResolvedVersion:  VersionString,
		Status:           http.StatusOK,
		GoModPath:        "",
		Error:            "",
	}
}

// Module creates a Module with the given path and version.
// The list of suffixes is used to create LegacyPackages within the module.
func Module(modulePath, version string, suffixes ...string) *internal.Module {
	mi := LegacyModuleInfo(modulePath, version)
	m := &internal.Module{
		LegacyModuleInfo: *mi,
		LegacyPackages:   nil,
		Licenses:         Licenses,
	}
	m.Units = []*internal.Unit{UnitForModuleRoot(mi, LicenseMetadata)}
	for _, s := range suffixes {
		lp := LegacyPackage(modulePath, s)
		if s != "" {
			AddPackage(m, lp)
		} else {
			m.LegacyPackages = append(m.LegacyPackages, lp)
			u := UnitForPackage(lp, modulePath, version)
			m.Units[0].Documentation = u.Documentation
		}
	}
	return m
}

func AddPackage(m *internal.Module, p *internal.LegacyPackage) *internal.Module {
	if m.ModulePath != stdlib.ModulePath && !strings.HasPrefix(p.Path, m.ModulePath) {
		panic(fmt.Sprintf("package path %q not a prefix of module path %q",
			p.Path, m.ModulePath))
	}
	m.LegacyPackages = append(m.LegacyPackages, p)
	AddUnit(m, UnitForPackage(p, m.ModulePath, m.Version))
	minLen := len(m.ModulePath)
	if m.ModulePath == stdlib.ModulePath {
		minLen = 1
	}
	for pth := p.Path; len(pth) > minLen; pth = path.Dir(pth) {
		found := false
		for _, u := range m.Units {
			if u.Path == pth {
				found = true
				break
			}
		}
		if !found {
			AddUnit(m, UnitEmpty(pth, m.ModulePath, m.Version))
		}
	}
	return m
}

func AddUnit(m *internal.Module, u *internal.Unit) {
	for _, e := range m.Units {
		if e.Path == u.Path {
			panic(fmt.Sprintf("module already has path %q", e.Path))
		}
	}
	m.Units = append(m.Units, u)
}

func AddLicense(m *internal.Module, lic *licenses.License) {
	m.Licenses = append(m.Licenses, lic)
	dir := path.Dir(lic.FilePath)
	if dir == "." {
		dir = ""
	}
	for _, u := range m.Units {
		if strings.TrimPrefix(u.Path, m.ModulePath+"/") == dir {
			u.Licenses = append(u.Licenses, lic.Metadata)
		}
	}
}

func UnitEmpty(path, modulePath, version string) *internal.Unit {
	return &internal.Unit{
		UnitMeta: *UnitMeta(path, modulePath, version, "", true),
	}
}

func UnitForModuleRoot(m *internal.LegacyModuleInfo, licenses []*licenses.Metadata) *internal.Unit {
	u := &internal.Unit{
		UnitMeta:        *UnitMeta(m.ModulePath, m.ModulePath, m.Version, "", m.IsRedistributable),
		LicenseContents: Licenses,
	}
	if m.LegacyReadmeFilePath != "" {
		u.Readme = &internal.Readme{
			Filepath: m.LegacyReadmeFilePath,
			Contents: m.LegacyReadmeContents,
		}
	}
	return u
}

func UnitForPackage(pkg *internal.LegacyPackage, modulePath, version string) *internal.Unit {
	return &internal.Unit{
		UnitMeta:        *UnitMeta(pkg.Path, modulePath, version, pkg.Name, pkg.IsRedistributable),
		Imports:         pkg.Imports,
		LicenseContents: Licenses,
		Documentation: &internal.Documentation{
			Synopsis: pkg.Synopsis,
			HTML:     pkg.DocumentationHTML,
			GOOS:     pkg.GOOS,
			GOARCH:   pkg.GOARCH,
		},
	}
}

func UnitMeta(path, modulePath, version, name string, isRedistributable bool) *internal.UnitMeta {
	return &internal.UnitMeta{
		ModulePath:        modulePath,
		Version:           version,
		Path:              path,
		Name:              name,
		IsRedistributable: isRedistributable,
		Licenses:          LicenseMetadata,
		SourceInfo:        source.NewGitHubInfo("https://"+modulePath, "", version),
	}
}

func constructFullPath(modulePath, suffix string) string {
	if modulePath != stdlib.ModulePath {
		return path.Join(modulePath, suffix)
	}
	return suffix
}
