package services

import "context"

// VersionProvider returns information about the running build.
type VersionProvider interface {
	Version(ctx context.Context) (string, error)
}

// StaticVersionService replies with the same version for every request.
type StaticVersionService struct {
	version string
}

// NewStaticVersionService creates a VersionProvider backed by a constant.
func NewStaticVersionService(version string) *StaticVersionService {
	return &StaticVersionService{version: version}
}

// Version returns the configured version string.
func (s *StaticVersionService) Version(_ context.Context) (string, error) {
	return s.version, nil
}
