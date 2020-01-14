// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package license

import (
	"archive/zip"
	"bytes"
	"math"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	lc "github.com/google/licensecheck"
	"golang.org/x/discovery/internal/testing/testhelper"
)

func TestDetect(t *testing.T) {
	cov := lc.Coverage{
		Percent: 100,
		Match:   []lc.Match{{Name: "MIT", Type: lc.MIT, Percent: 100}},
	}
	testCases := []struct {
		name, subdir string
		contents     map[string]string
		want         []*Metadata
	}{
		{
			name: "valid license",
			contents: map[string]string{
				"foo/LICENSE": testhelper.MITLicense,
			},
			want: []*Metadata{{Types: []string{"MIT"}, FilePath: "foo/LICENSE", Coverage: cov}},
		},
		{
			name: "valid license, british spelling",
			contents: map[string]string{
				"foo/LICENCE": testhelper.MITLicense,
			},
			want: []*Metadata{{Types: []string{"MIT"}, FilePath: "foo/LICENCE", Coverage: cov}},
		},
		{
			name: "valid license md format",
			contents: map[string]string{
				"foo/LICENSE.md": testhelper.MITLicense,
			},
			want: []*Metadata{{Types: []string{"MIT"}, FilePath: "foo/LICENSE.md", Coverage: cov}},
		},
		{
			name: "valid license trim prefix",
			contents: map[string]string{
				"rsc.io/quote@v1.4.1/LICENSE.md": testhelper.MITLicense,
			},
			subdir: "rsc.io/quote@v1.4.1",
			want:   []*Metadata{{Types: []string{"MIT"}, FilePath: "LICENSE.md", Coverage: cov}},
		},
		{
			name: "multiple licenses",
			contents: map[string]string{
				"LICENSE":        testhelper.MITLicense,
				"bar/LICENSE.md": testhelper.MITLicense,
				"foo/COPYING":    testhelper.BSD0License,
			},
			want: []*Metadata{
				{Types: []string{"MIT"}, FilePath: "LICENSE", Coverage: cov},
				{Types: []string{"MIT"}, FilePath: "bar/LICENSE.md", Coverage: cov},
				{Types: []string{"BSD-0-Clause"}, FilePath: "foo/COPYING", Coverage: lc.Coverage{
					Percent: 100,
					Match:   []lc.Match{{Name: "BSD-0-Clause", Type: lc.BSD, Percent: 100}},
				}},
			},
		},
		{
			name: "multiple licenses in a single file",
			contents: map[string]string{
				"LICENSE": testhelper.MITLicense + "\n" + testhelper.BSD0License,
			},
			want: []*Metadata{
				{Types: []string{"BSD-0-Clause", "MIT"}, FilePath: "LICENSE", Coverage: lc.Coverage{
					Percent: 100,
					Match: []lc.Match{
						{Name: "MIT", Type: lc.MIT, Percent: 100},
						{Name: "BSD-0-Clause", Type: lc.BSD, Percent: 100},
					},
				}},
			},
		},
		{
			name: "unknown license",
			contents: map[string]string{
				"LICENSE": testhelper.UnknownLicense,
			},
			want: []*Metadata{
				{FilePath: "LICENSE"},
			},
		},
		{
			name: "low coverage license",
			contents: map[string]string{
				"LICENSE": testhelper.MITLicense + `
Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod
tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim
veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea
commodo consequat.`,
			},
			want: []*Metadata{
				{
					FilePath: "LICENSE",
					Coverage: lc.Coverage{
						Percent: 81.9095,
						Match:   []lc.Match{{Name: "MIT", Type: lc.MIT, Percent: 100}},
					},
				},
			},
		},
		{
			name: "no license",
			contents: map[string]string{
				"foo/blah.go": "package foo\n\nconst Foo = 42",
			},
		},
		{
			name: "invalid license file name",
			contents: map[string]string{
				"MYLICENSEFILE": testhelper.MITLicense,
			},
		},
		{
			name: "ignores licenses in vendored packages, but not packages named vendor",
			contents: map[string]string{
				"pkg/vendor/LICENSE": testhelper.MITLicense,
				"vendor/pkg/LICENSE": testhelper.MITLicense,
			},
			want: []*Metadata{
				{Types: []string{"MIT"}, FilePath: "pkg/vendor/LICENSE", Coverage: cov},
			},
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			var (
				z   *zip.Reader
				err error
			)
			if test.contents != nil {
				zipBytes, err := testhelper.ZipContents(test.contents)
				if err != nil {
					t.Fatal(err)
				}
				z, err = zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
				if err != nil {
					t.Fatal(err)
				}
			}
			got, err := Detect(test.subdir, z)
			if err != nil {
				t.Error(err)
			}
			sort.Slice(got, func(i, j int) bool {
				if got[i].FilePath < got[j].FilePath {
					return true
				}
				return got[i].FilePath < got[j].FilePath
			})
			var gotFiles []*Metadata
			for _, l := range got {
				gotFiles = append(gotFiles, l.Metadata)
			}

			opts := []cmp.Option{
				cmp.Comparer(coveragePercentEqual),
				cmpopts.IgnoreFields(lc.Match{}, "Start", "End"),
			}
			if diff := cmp.Diff(test.want, gotFiles, opts...); diff != "" {
				t.Errorf("detectLicense(z) mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Treat two coverage percentages as the same if they are within 4 percentage points,
// and both are on the same side of 90% (our threshold).
func coveragePercentEqual(a, b float64) bool {
	if (a >= 90) != (b >= 90) {
		return false
	}
	return math.Abs(a-b) <= 4
}
