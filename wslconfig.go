package main

import (
	"os/user"
	"path"
)

// @TOD: wsl2Kernel, err := parseWSLConfig()
// parse <home>/.wslconfig if exists - use INI package
// determine kernel dir (if not set via a flag). Use default if neither is defined
// if there is a kernel key in the wsl2 section, use that for 'current' digest

// returns the default location of WSL configuration file
func wslConfigPath() (string, error) {
	home, err := userHomeDirectory()
	if err != nil {
		return "", err
	}
	return path.Join(home, ".wslconfig"), nil
}

// returns the home directory for the current user
func userHomeDirectory() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.HomeDir, nil
}
