/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package dfc

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMapImage(t *testing.T) {
	images := map[string]string{
		// Simple name match
		"alpine": "chainguard-base:latest",
		// Full registry path
		"gcr.io/kaniko-project/executor": "kaniko",
		// Docker Hub library shorthand (docker.io/library/...)
		"library/docker": "docker-dind",
		// Glob pattern
		"golang*": "go",
		// Mapping that includes a tag override
		"node:18-slim": "node:18-dev",
	}

	tests := []struct {
		name        string
		base        string
		tag         string
		wantImage   string
		wantTagOver string
		wantOK      bool
	}{
		{
			name:        "exact match with tag override in mapping",
			base:        "alpine",
			tag:         "",
			wantImage:   "chainguard-base",
			wantTagOver: "latest",
			wantOK:      true,
		},
		{
			name:        "exact match preserves caller tag when no override",
			base:        "gcr.io/kaniko-project/executor",
			tag:         "v1.9.0",
			wantImage:   "kaniko",
			wantTagOver: "",
			wantOK:      true,
		},
		{
			name:        "docker hub library variant",
			base:        "docker.io/library/docker",
			tag:         "",
			wantImage:   "docker-dind",
			wantTagOver: "",
			wantOK:      true,
		},
		{
			name:        "glob pattern match",
			base:        "golang",
			tag:         "1.21",
			wantImage:   "go",
			wantTagOver: "",
			wantOK:      true,
		},
		{
			name:        "glob pattern match with longer name",
			base:        "golang",
			tag:         "1.21-alpine",
			wantImage:   "go",
			wantTagOver: "",
			wantOK:      true,
		},
		{
			name:        "exact match on full image ref including tag",
			base:        "node",
			tag:         "18-slim",
			wantImage:   "node",
			wantTagOver: "18-dev",
			wantOK:      true,
		},
		{
			name:        "no match returns empty strings and false",
			base:        "someunknownimage",
			tag:         "1.0",
			wantImage:   "",
			wantTagOver: "",
			wantOK:      false,
		},
		{
			name:        "no match with registry prefix returns false",
			base:        "example.com/myorg/myimage",
			tag:         "",
			wantImage:   "",
			wantTagOver: "",
			wantOK:      false,
		},
		{
			name:        "base filename used when full path has no match",
			base:        "gcr.io/kaniko-project/executor",
			tag:         "",
			wantImage:   "kaniko",
			wantTagOver: "",
			wantOK:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotImage, gotTagOver, gotOK := MapImage(images, tt.base, tt.tag)
			if gotOK != tt.wantOK {
				t.Errorf("MapImage(%q, %q) ok = %v, want %v", tt.base, tt.tag, gotOK, tt.wantOK)
			}
			if gotImage != tt.wantImage {
				t.Errorf("MapImage(%q, %q) image = %q, want %q", tt.base, tt.tag, gotImage, tt.wantImage)
			}
			if gotTagOver != tt.wantTagOver {
				t.Errorf("MapImage(%q, %q) overrideTag = %q, want %q", tt.base, tt.tag, gotTagOver, tt.wantTagOver)
			}
		})
	}
}

func TestMapPackage(t *testing.T) {
	packages := PackageMap{
		DistroDebian: {
			"libssl-dev": {"libssl3"},
			"ssh":        {"openssh-client", "openssh-server"},
			"python3":    {"python-3"},
			// Packages that map to nothing (explicitly empty)
			"software-properties-common": {},
		},
		DistroFedora: {
			"apr": {"apr-util"},
		},
		DistroAlpine: {},
	}

	tests := []struct {
		name   string
		distro Distro
		pkg    string
		want   []string
	}{
		{
			name:   "debian single mapping",
			distro: DistroDebian,
			pkg:    "libssl-dev",
			want:   []string{"libssl3"},
		},
		{
			name:   "debian multiple mappings",
			distro: DistroDebian,
			pkg:    "ssh",
			want:   []string{"openssh-client", "openssh-server"},
		},
		{
			name:   "debian explicit empty mapping",
			distro: DistroDebian,
			pkg:    "software-properties-common",
			want:   []string{},
		},
		{
			name:   "fedora package",
			distro: DistroFedora,
			pkg:    "apr",
			want:   []string{"apr-util"},
		},
		{
			name:   "unknown package returns nil",
			distro: DistroDebian,
			pkg:    "notapackage",
			want:   nil,
		},
		{
			name:   "unknown distro returns nil",
			distro: Distro("windows"),
			pkg:    "libssl-dev",
			want:   nil,
		},
		{
			name:   "known distro with no packages returns nil",
			distro: DistroAlpine,
			pkg:    "curl",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapPackage(packages, tt.distro, tt.pkg)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("MapPackage(%q, %q) mismatch (-want +got):\n%s", tt.distro, tt.pkg, diff)
			}
		})
	}
}
