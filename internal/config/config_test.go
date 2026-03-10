package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreLoadMissingFile(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DefaultCatalogName != "Homelab Static Hosts" {
		t.Fatalf("Load() DefaultCatalogName = %q, want %q", cfg.DefaultCatalogName, "Homelab Static Hosts")
	}
	if cfg.DefaultHostSetName != "linux-servers" {
		t.Fatalf("Load() DefaultHostSetName = %q, want %q", cfg.DefaultHostSetName, "linux-servers")
	}
	if cfg.DefaultRoleName != "linux-ssh-users" {
		t.Fatalf("Load() DefaultRoleName = %q, want %q", cfg.DefaultRoleName, "linux-ssh-users")
	}
	if cfg.DefaultTargetPort != 22 {
		t.Fatalf("Load() DefaultTargetPort = %d, want 22", cfg.DefaultTargetPort)
	}
	if !cfg.DefaultCreateRole {
		t.Fatalf("Load() DefaultCreateRole = %v, want true", cfg.DefaultCreateRole)
	}
	if !cfg.DefaultConnectAfterCreate {
		t.Fatalf("Load() DefaultConnectAfterCreate = %v, want true", cfg.DefaultConnectAfterCreate)
	}
}

func TestStoreSaveAndLoadRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	want := Config{
		BoundaryAddr:              "https://boundary.example.com",
		OIDCAuthMethodID:          "amoidc_1234",
		AuthToken:                 "at_exampletoken",
		DefaultProjectScopeID:     "p_1234567890",
		DefaultCatalogName:        "Homelab",
		DefaultHostSetName:        "linux",
		DefaultRoleName:           "ssh-users",
		DefaultPrincipalID:        "mgoidc_1234",
		DefaultTargetPort:         2222,
		DefaultCreateRole:         false,
		DefaultConnectAfterCreate: false,
		AutoSelectDefaultScope:    true,
		LastHostCatalogID:         "hcst_1234",
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got != want {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestStoreLoadUsesEnvironmentOverrides(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if err := store.Save(Config{BoundaryAddr: "https://from-file.example.com", AuthToken: "at_from_file", DefaultTargetPort: 22, DefaultCatalogName: "Homelab Static Hosts", DefaultHostSetName: "linux-servers", DefaultRoleName: "linux-ssh-users", DefaultCreateRole: true, DefaultConnectAfterCreate: true}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	t.Setenv("BNDRY_BOUNDARY_ADDR", "https://from-env.example.com")
	t.Setenv("BNDRY_AUTH_TOKEN", "at_from_env")
	t.Setenv("BNDRY_DEFAULT_TARGET_PORT", "2022")

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.BoundaryAddr != "https://from-env.example.com" {
		t.Fatalf("Load() BoundaryAddr = %q, want env override", got.BoundaryAddr)
	}
	if got.AuthToken != "at_from_env" {
		t.Fatalf("Load() AuthToken = %q, want env override", got.AuthToken)
	}
	if got.DefaultTargetPort != 2022 {
		t.Fatalf("Load() DefaultTargetPort = %d, want 2022", got.DefaultTargetPort)
	}
}

func TestStoreInitCreatesConfigFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if err := store.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}

	if err := store.Init(); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second Init() error = %v, want os.ErrExist", err)
	}
}
