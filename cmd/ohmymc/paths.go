package main

import (
	"os"
	"path/filepath"
)

type launcherPaths struct {
	Data  string
	Games string
}

func resolvePaths(data, games string) (launcherPaths, error) {
	if data == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return launcherPaths{}, err
		}
		data = filepath.Join(base, "OhMyMC")
	}
	if games == "" {
		executable, err := os.Executable()
		if err != nil {
			return launcherPaths{}, err
		}
		// Anchor to the executable, not a shortcut's working directory.
		games = filepath.Join(filepath.Dir(executable), ".minecraft")
	}
	data, err := filepath.Abs(data)
	if err != nil {
		return launcherPaths{}, err
	}
	games, err = filepath.Abs(games)
	if err != nil {
		return launcherPaths{}, err
	}
	return launcherPaths{Data: data, Games: games}, nil
}
