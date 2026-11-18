//go:build !darwin

package icon

import "context"

// FromBundle reads an installed app's icon; only macOS has bundles so far.
func FromBundle(context.Context, string) (Icon, error) { return Icon{}, ErrNone }
