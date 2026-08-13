// Package config resolves vexillologist settings from flags, environment
// variables, and the config file, in that precedence order.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// File is the on-disk config at ~/.config/vexillologist/config.yaml.
type File struct {
	// Token is supported but discouraged; prefer the LD_API_TOKEN env var.
	Token       string `yaml:"token,omitempty"`
	Project     string `yaml:"project,omitempty"`
	Environment string `yaml:"environment,omitempty"`
	// GuardedEnvironments are extra env keys (beyond LD's critical flag and
	// prod/production) that require strong confirmation before mutations.
	GuardedEnvironments []string `yaml:"guarded_environments,omitempty"`
}

// Resolved is the effective configuration after applying precedence.
type Resolved struct {
	Token       string
	TokenSource string // "flag", "env:<NAME>", or "config file"
	Project     string
	Environment string
	Guarded     []string
}

// Flags are the values supplied on the command line (highest precedence).
type Flags struct {
	Token       string
	Project     string
	Environment string
}

// tokenEnvVars in precedence order. LD_API_TOKEN is our documented primary;
// LD_ACCESS_TOKEN is ldcli's convention.
var tokenEnvVars = []string{"LD_API_TOKEN", "LD_ACCESS_TOKEN", "LAUNCHDARKLY_API_TOKEN"}

// Path returns the config file location, honoring os.UserConfigDir.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vexillologist", "config.yaml"), nil
}

// Load reads the config file; a missing file yields a zero File, not an error.
func Load() (File, error) {
	p, err := Path()
	if err != nil {
		return File{}, err
	}
	return loadFrom(p)
}

func loadFrom(path string) (File, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return File{}, nil
	}
	if err != nil {
		return File{}, err
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return File{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return f, nil
}

// Save writes the config file with 0600 permissions, creating the directory
// as needed.
func Save(f File) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// Resolve applies precedence: flags > env vars > config file.
func Resolve(flags Flags, file File) Resolved {
	r := Resolved{
		Project:     file.Project,
		Environment: file.Environment,
		Guarded:     file.GuardedEnvironments,
	}

	if file.Token != "" {
		r.Token, r.TokenSource = file.Token, "config file"
	}
	for i := len(tokenEnvVars) - 1; i >= 0; i-- {
		if v := os.Getenv(tokenEnvVars[i]); v != "" {
			r.Token, r.TokenSource = v, "env:"+tokenEnvVars[i]
		}
	}
	if flags.Token != "" {
		r.Token, r.TokenSource = flags.Token, "flag"
	}

	if v := os.Getenv("LD_PROJECT"); v != "" {
		r.Project = v
	}
	if flags.Project != "" {
		r.Project = flags.Project
	}

	if v := os.Getenv("LD_ENVIRONMENT"); v != "" {
		r.Environment = v
	}
	if flags.Environment != "" {
		r.Environment = flags.Environment
	}

	return r
}
