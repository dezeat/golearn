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

package ports

import "github.com/dezeat/golearn/internal/domain"

// PackReader reads question packs from files.
type PackReader interface {
	// ReadPack parses a pack file at the given path.
	ReadPack(path string) (*domain.Pack, error)
	// ReadPackFromBytes parses pack content from an in-memory byte slice.
	// filename is used solely for error messages and extension detection.
	ReadPackFromBytes(data []byte, filename string) (*domain.Pack, error)
}
