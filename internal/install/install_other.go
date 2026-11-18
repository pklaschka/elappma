//go:build !darwin

package install

import "context"

// New returns an installer that refuses: Windows and Linux come later and
// implement Installer in their own files.
func New(Options) Installer { return unsupported{} }

type unsupported struct{}

func (unsupported) Install(context.Context, string, string) (Result, error) {
	return Result{}, ErrUnsupported
}

func (unsupported) Uninstall(context.Context, string) error { return ErrUnsupported }

func (unsupported) InstalledVersion(string) (string, string) { return "", "" }

func (unsupported) Open(context.Context, string) error { return ErrUnsupported }
