package config

import (
	"os"
	"path/filepath"
	"testing"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, v := range []string{"LD_API_TOKEN", "LD_ACCESS_TOKEN", "LAUNCHDARKLY_API_TOKEN", "LD_PROJECT", "LD_ENVIRONMENT"} {
		t.Setenv(v, "")
	}
}

func TestResolvePrecedence(t *testing.T) {
	file := File{
		Token:       "file-token",
		Project:     "file-proj",
		Environment: "file-env",
	}

	t.Run("file only", func(t *testing.T) {
		clearEnv(t)
		r := Resolve(Flags{}, file)
		if r.Token != "file-token" || r.TokenSource != "config file" {
			t.Errorf("token = %q from %q", r.Token, r.TokenSource)
		}
		if r.Project != "file-proj" || r.Environment != "file-env" {
			t.Errorf("project/env = %q/%q", r.Project, r.Environment)
		}
	})

	t.Run("env beats file", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("LD_API_TOKEN", "env-token")
		t.Setenv("LD_PROJECT", "env-proj")
		t.Setenv("LD_ENVIRONMENT", "env-env")
		r := Resolve(Flags{}, file)
		if r.Token != "env-token" || r.TokenSource != "env:LD_API_TOKEN" {
			t.Errorf("token = %q from %q", r.Token, r.TokenSource)
		}
		if r.Project != "env-proj" || r.Environment != "env-env" {
			t.Errorf("project/env = %q/%q", r.Project, r.Environment)
		}
	})

	t.Run("LD_API_TOKEN beats aliases", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("LAUNCHDARKLY_API_TOKEN", "alias2")
		t.Setenv("LD_ACCESS_TOKEN", "alias1")
		t.Setenv("LD_API_TOKEN", "primary")
		r := Resolve(Flags{}, File{})
		if r.Token != "primary" {
			t.Errorf("token = %q, want primary", r.Token)
		}
	})

	t.Run("ldcli alias works alone", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("LD_ACCESS_TOKEN", "ldcli-token")
		r := Resolve(Flags{}, File{})
		if r.Token != "ldcli-token" || r.TokenSource != "env:LD_ACCESS_TOKEN" {
			t.Errorf("token = %q from %q", r.Token, r.TokenSource)
		}
	})

	t.Run("flags beat everything", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("LD_API_TOKEN", "env-token")
		t.Setenv("LD_PROJECT", "env-proj")
		r := Resolve(Flags{Token: "flag-token", Project: "flag-proj", Environment: "flag-env"}, file)
		if r.Token != "flag-token" || r.TokenSource != "flag" {
			t.Errorf("token = %q from %q", r.Token, r.TokenSource)
		}
		if r.Project != "flag-proj" || r.Environment != "flag-env" {
			t.Errorf("project/env = %q/%q", r.Project, r.Environment)
		}
	})
}

func TestLoadFrom(t *testing.T) {
	t.Run("missing file is not an error", func(t *testing.T) {
		f, err := loadFrom(filepath.Join(t.TempDir(), "nope.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if f.Project != "" || f.Environment != "" || f.Token != "" || len(f.GuardedEnvironments) != 0 {
			t.Errorf("f = %+v, want zero", f)
		}
	})

	t.Run("round trip", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		data := "project: my-proj\nenvironment: staging\nguarded_environments:\n  - demo\n"
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		f, err := loadFrom(path)
		if err != nil {
			t.Fatal(err)
		}
		if f.Project != "my-proj" || f.Environment != "staging" {
			t.Errorf("f = %+v", f)
		}
		if len(f.GuardedEnvironments) != 1 || f.GuardedEnvironments[0] != "demo" {
			t.Errorf("guarded = %v", f.GuardedEnvironments)
		}
	})

	t.Run("malformed yaml errors", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte("{not yaml"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadFrom(path); err == nil {
			t.Error("want parse error, got nil")
		}
	})
}
