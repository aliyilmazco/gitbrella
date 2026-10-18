//go:build !darwin

package scan

type noDataless struct{}

func (noDataless) IsDataless(string) (bool, error) { return false, nil }

// DefaultFlagReader reports no dataless files on systems without them.
func DefaultFlagReader() FlagReader { return noDataless{} }

// PreventMaterialization is a no-op outside macOS.
func PreventMaterialization() error { return nil }

// AllowMaterialization runs fn: there is no policy to lift outside macOS.
func AllowMaterialization(fn func() error) error { return fn() }
