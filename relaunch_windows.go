package main

import "errors"

// reopenAfterExit is not needed until Elappma installs itself on Windows.
func reopenAfterExit(string) error { return errors.New("not supported on Windows yet") }
