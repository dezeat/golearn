// Copyright 2026 dezeat
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package pack implements reading question pack files (YAML / JSON).
package pack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dezeat/golearn/internal/domain"
)

// Reader reads pack files from disk.
type Reader struct{}

// NewReader creates a new pack reader.
func NewReader() *Reader {
	return &Reader{}
}

// ReadPack parses a single YAML or JSON pack file.
func (r *Reader) ReadPack(path string) (*domain.Pack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pack file %s: %w", path, err)
	}
	return r.ReadPackFromBytes(data, path)
}

// ReadPackFromBytes parses pack content from an in-memory byte slice.
// filename is used for error messages and to detect the format via its
// extension (.yaml / .yml → YAML, .json → JSON).
func (r *Reader) ReadPackFromBytes(data []byte, filename string) (*domain.Pack, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	var pack domain.Pack

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &pack); err != nil {
			return nil, fmt.Errorf("parse YAML %s: %w", filename, err)
		}
	case ".json":
		if err := json.Unmarshal(data, &pack); err != nil {
			return nil, fmt.Errorf("parse JSON %s: %w", filename, err)
		}
	default:
		return nil, fmt.Errorf("unsupported file extension %q (expected .yaml, .yml, or .json)", ext)
	}

	return &pack, nil
}

// ReadDirectory reads all pack files (*.yaml, *.yml, *.json) from a directory.
func (r *Reader) ReadDirectory(dir string) ([]*domain.Pack, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{fmt.Errorf("read directory %s: %w", dir, err)}
	}

	var packs []*domain.Pack
	var errs []error

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			continue
		}
		p, err := r.ReadPack(filepath.Join(dir, entry.Name()))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		packs = append(packs, p)
	}

	return packs, errs
}
