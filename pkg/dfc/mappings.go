/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package dfc

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/chainguard-dev/clog"
	"gopkg.in/yaml.v3"
)

//go:embed builtin-mappings.yaml
var builtinMappingsYAMLBytes []byte

// GetDefaultMappings gets the default mappings
func GetDefaultMappings(ctx context.Context, update bool) (MappingsConfig, error) {
	return defaultGetDefaultMappings(ctx, update)
}

// defaultGetDefaultMappings is the real implementation of GetDefaultMappings
func defaultGetDefaultMappings(ctx context.Context, update bool) (MappingsConfig, error) {
	log := clog.FromContext(ctx)
	var mappings MappingsConfig

	// If update is requested, try to update the mappings first
	if update {
		// Set up update options
		updateOpts := UpdateOptions{}
		// Use the default URL
		updateOpts.MappingsURL = defaultMappingsURL

		if err := Update(ctx, updateOpts); err != nil {
			log.Warn("Failed to update mappings, will try to use existing mappings", "error", err)
		}
	}

	// Try to use XDG config mappings file if available
	xdgMappings, err := getMappingsConfig()
	if err != nil {
		return mappings, fmt.Errorf("checking XDG config mappings: %w", err)
	}

	var mappingsBytes []byte
	if xdgMappings != nil {
		log.Debug("Using mappings from XDG config directory")
		mappingsBytes = xdgMappings
	} else {
		// Fall back to embedded mappings
		log.Debug("Using embedded builtin mappings")
		mappingsBytes = builtinMappingsYAMLBytes
	}

	// Unmarshal the mappings
	if err := yaml.Unmarshal(mappingsBytes, &mappings); err != nil {
		return mappings, fmt.Errorf("unmarshalling mappings: %w", err)
	}

	return mappings, nil
}

// MergeMappings merges the base and overlay mappings
// Any values in the overlay take precedence over the base
func MergeMappings(base, overlay MappingsConfig) MappingsConfig {
	result := MappingsConfig{
		Images:   make(map[string]string),
		Packages: make(PackageMap),
	}

	// Copy base images
	for k, v := range base.Images {
		result.Images[k] = v
	}

	// Overlay with extra images
	for k, v := range overlay.Images {
		result.Images[k] = v
	}

	// Copy base packages for each distro
	for distro, packages := range base.Packages {
		if result.Packages[distro] == nil {
			result.Packages[distro] = make(map[string][]string)
		}
		for pkg, mappings := range packages {
			result.Packages[distro][pkg] = mappings
		}
	}

	// Overlay with extra packages
	for distro, packages := range overlay.Packages {
		if result.Packages[distro] == nil {
			result.Packages[distro] = make(map[string][]string)
		}
		for pkg, mappings := range packages {
			result.Packages[distro][pkg] = mappings
		}
	}

	return result
}

// MapPackage looks up a package name in the provided mappings for a given distro
// and returns the mapped Chainguard package names. Returns nil if no mapping exists.
func MapPackage(packages PackageMap, distro Distro, name string) []string {
	if distroMap, exists := packages[distro]; exists {
		if mapped, ok := distroMap[name]; ok {
			return mapped
		}
	}
	return nil
}

// MapImage maps an image to Chainguard with the provided mappings. It returns
// the mapped image name, an optional tag override, and whether a mapping was
// found. If no mapping is found, all return values are empty strings and false.
func MapImage(images map[string]string, base, tag string) (image, overrideTag string, ok bool) {
	baseFilename := filepath.Base(base)

	var mappedImage string

	// Exact match with full image reference including tag
	fullImageRef := base
	if tag != "" {
		fullImageRef += ":" + tag
	}
	if img, found := images[fullImageRef]; found {
		mappedImage = img
	} else if img, found := images[base]; found {
		mappedImage = img
	} else if img, found := images[baseFilename]; found {
		mappedImage = img
	} else {
		// Check Docker Hub registry variants (e.g. docker.io/library/node, index.docker.io/node)
		for _, variant := range generateDockerHubVariants(base) {
			if img, found := images[variant]; found {
				mappedImage = img
				break
			}
		}

		// Normalize and retry (strips well-known registry prefixes)
		if mappedImage == "" {
			normalizedBase := normalizeImageName(base)
			if img, found := images[normalizedBase]; found {
				mappedImage = img
			} else if strings.HasPrefix(normalizedBase, "library/") {
				simpleBase := strings.TrimPrefix(normalizedBase, "library/")
				if img, found := images[simpleBase]; found {
					mappedImage = img
				}
			}
		}

		// Glob patterns (e.g. "golang*" → "go")
		if mappedImage == "" {
			for pattern, img := range images {
				if strings.HasSuffix(pattern, "*") {
					prefix := strings.TrimSuffix(pattern, "*")
					if strings.HasPrefix(baseFilename, prefix) {
						mappedImage = img
						break
					}
				}
			}
		}
	}

	if mappedImage == "" {
		return "", "", false
	}

	if parts := strings.Split(mappedImage, ":"); len(parts) > 1 {
		image = parts[0]
		overrideTag = parts[1]
	} else {
		image = mappedImage
	}

	return image, overrideTag, true
}
