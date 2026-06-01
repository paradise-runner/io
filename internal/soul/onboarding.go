package soul

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// ChoiceFunc obtains an onboarding Choice (e.g. via an interactive picker).
type ChoiceFunc func() (Choice, error)

// EnsureSoul writes a SOUL.md at soulPath if one does not already exist, using
// the Choice obtained from choose. It returns true if it created the file. If
// SOUL.md already exists, choose is not called and created is false.
func EnsureSoul(soulPath string, choose ChoiceFunc) (created bool, err error) {
	if _, statErr := os.Stat(soulPath); statErr == nil {
		return false, nil
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return false, statErr
	}

	choice, err := choose()
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(soulPath), 0o700); err != nil {
		return false, err
	}
	if err := os.WriteFile(soulPath, []byte(Render(choice)), 0o600); err != nil {
		return false, err
	}
	return true, nil
}
