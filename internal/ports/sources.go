package ports

import "github.com/dezeat/golearn/internal/domain"

// PackReader reads question packs from files.
type PackReader interface {
	// ReadPack parses a pack file at the given path.
	ReadPack(path string) (*domain.Pack, error)
}
