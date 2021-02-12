package main

import (
	"os"
	"os/user"
	"path"

	"gopkg.in/ini.v1"
)

const (
	wslConfigFile = ".wslconfig"
	wsl2Section   = "wsl2"
	wsl2KernelKey = "kernel"
)

// return the configured kernel path. Returns empty string if undefined
func wslConfigGetKernelPath() (string, error) {
	cfg, err := wslConfigLoad()
	if err != nil {
		return "", err
	}
	return cfg.Section(wsl2Section).Key(wsl2KernelKey).String(), nil
}

// sets the configured kernel path, creating the configuration file if needed
func wslConfigSetKernel(kernel string) error {
	cfg, err := wslConfigLoad()

	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err != nil {
		cfg = ini.Empty() // create an empty configuration
	}

	ini.PrettyFormat = false // don't align '=' across keys
	ini.PrettyEqual = true   // but keep spaces around the '=' sign
	cfg.Section(wsl2Section).Key(wsl2KernelKey).SetValue(kernel)
	filename, _ := wslConfigFilePath()
	err = cfg.SaveTo(filename)
	return err
}

// returns the default location of WSL configuration file
func wslConfigLoad() (*ini.File, error) {
	cfg, err := wslConfigFilePath()
	if err != nil {
		return nil, err
	}

	// control type of error returned when wslconfig doesn't exist.
	// the call to ini.Load() could return arbitrary error
	if _, err = os.Stat(cfg); os.IsNotExist(err) {
		return nil, err
	}
	return ini.Load(cfg)
}

// returns the (default) WSL configuration file path
func wslConfigFilePath() (string, error) {
	home, err := userHomeDirectory()
	if err != nil {
		return "", err
	}
	return path.Join(home, wslConfigFile), nil
}

// returns the home directory path of the current user
func userHomeDirectory() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.HomeDir, nil
}
