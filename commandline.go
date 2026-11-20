package main

import (
	"context"
	"errors"
	"time"

	"github.com/pklaschka/lieferling/internal/install"
)

// CommandLine says whether the lieferling command is set up in the terminal.
func (s *AppService) CommandLine() install.Command {
	return install.CommandStatus(install.BundledCommand(s.self), install.CommandPath)
}

// commandTimeout bounds the wait for the password dialog, so the window
// never waits for one that macOS did not show.
const commandTimeout = 2 * time.Minute

// InstallCommandLine links the command line in this app into the PATH,
// replacing an older copy there. macOS asks for an administrator password;
// closing that dialog changes nothing and is no error.
func (s *AppService) InstallCommandLine(ctx context.Context) (install.Command, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	bundled := install.BundledCommand(s.self)
	return s.commandDone(ctx, install.LinkCommand(ctx, bundled, install.CommandPath, false))
}

// RemoveCommandLine takes the link out of the PATH again.
func (s *AppService) RemoveCommandLine(ctx context.Context) (install.Command, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	return s.commandDone(ctx, install.UnlinkCommand(ctx, install.CommandPath, false))
}

func (s *AppService) commandDone(ctx context.Context, err error) (install.Command, error) {
	switch {
	case err == nil, errors.Is(err, install.ErrCanceled), errors.Is(ctx.Err(), context.Canceled):
		return s.CommandLine(), nil
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return s.CommandLine(), errors.New("macOS hat nicht rechtzeitig nach dem Passwort gefragt. Richte den Befehl stattdessen im Terminal ein.")
	default:
		return s.CommandLine(), translate(err)
	}
}
