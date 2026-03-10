package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const fileMode = 0o600

var supportedConfigNames = []string{
	"config.yaml",
	"config.yml",
	"config.json",
	"config.toml",
	"config.hcl",
}

// Config stores universal defaults and saved values for bndry.
type Config struct {
	BoundaryAddr              string `json:"boundary_addr" mapstructure:"boundary_addr"`
	OIDCAuthMethodID          string `json:"oidc_auth_method_id" mapstructure:"oidc_auth_method_id"`
	AuthToken                 string `json:"auth_token,omitempty" mapstructure:"auth_token"`
	DefaultProjectScopeID     string `json:"default_project_scope_id" mapstructure:"default_project_scope_id"`
	DefaultCatalogName        string `json:"default_catalog_name" mapstructure:"default_catalog_name"`
	DefaultHostSetName        string `json:"default_host_set_name" mapstructure:"default_host_set_name"`
	DefaultRoleName           string `json:"default_role_name" mapstructure:"default_role_name"`
	DefaultPrincipalID        string `json:"default_principal_id,omitempty" mapstructure:"default_principal_id"`
	DefaultTargetPort         int    `json:"default_target_port" mapstructure:"default_target_port"`
	DefaultCreateRole         bool   `json:"default_create_role" mapstructure:"default_create_role"`
	DefaultConnectAfterCreate bool   `json:"default_connect_after_create" mapstructure:"default_connect_after_create"`
	AutoSelectDefaultScope    bool   `json:"auto_select_default_scope" mapstructure:"auto_select_default_scope"`
	LastHostCatalogID         string `json:"last_host_catalog_id,omitempty" mapstructure:"last_host_catalog_id"`
}

// Store persists bndry configuration on disk.
type Store struct {
	path string
}

// DefaultConfig returns the built-in defaults for bndry configuration.
func DefaultConfig() Config {
	return Config{
		DefaultCatalogName:        "Homelab Static Hosts",
		DefaultHostSetName:        "linux-servers",
		DefaultRoleName:           "linux-ssh-users",
		DefaultTargetPort:         22,
		DefaultCreateRole:         true,
		DefaultConnectAfterCreate: true,
		AutoSelectDefaultScope:    false,
	}
}

// DefaultPath returns the preferred config file path for bndry.
//
// If BNDRY_CONFIG is set, that path is used. Otherwise, bndry will reuse the
// first existing config file it finds in ~/.config/bndry/, falling back to
// config.yaml when nothing exists yet.
func DefaultPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv("BNDRY_CONFIG")); override != "" {
		return override, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}

	baseDir := filepath.Join(configDir, "bndry")
	for _, name := range supportedConfigNames {
		candidate := filepath.Join(baseDir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return filepath.Join(baseDir, supportedConfigNames[0]), nil
}

// NewStore creates a config store. If path is empty, the default path is used.
func NewStore(path string) (*Store, error) {
	if path == "" {
		defaultPath, err := DefaultPath()
		if err != nil {
			return nil, err
		}
		path = defaultPath
	}

	return &Store{path: path}, nil
}

// Path returns the fully-qualified location of the config file.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}

	return s.path
}

// Load reads the effective configuration using Viper defaults, config files,
// and environment overrides.
func (s *Store) Load() (Config, error) {
	if s == nil {
		return Config{}, errors.New("config store is nil")
	}

	v := newViper(s.path)
	if _, err := os.Stat(s.path); err == nil {
		if err := v.ReadInConfig(); err != nil {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("stat config: %w", err)
	}

	cfg := DefaultConfig()
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

// Init writes a new config file populated with the built-in defaults.
func (s *Store) Init() error {
	if s == nil {
		return errors.New("config store is nil")
	}

	if _, err := os.Stat(s.path); err == nil {
		return fmt.Errorf("config already exists: %w", os.ErrExist)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat config: %w", err)
	}

	return s.Save(DefaultConfig())
}

// Save writes the config file with secure permissions.
func (s *Store) Save(cfg Config) error {
	if s == nil {
		return errors.New("config store is nil")
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	v := newViper(s.path)
	applyConfig(v, cfg)

	if err := v.WriteConfigAs(s.path); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	if err := os.Chmod(s.path, fileMode); err != nil {
		return fmt.Errorf("set config permissions: %w", err)
	}

	return nil
}

func newViper(path string) *viper.Viper {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("BNDRY")
	v.AutomaticEnv()
	applyConfigDefaults(v)
	return v
}

func applyConfigDefaults(v *viper.Viper) {
	defaults := DefaultConfig()
	v.SetDefault("boundary_addr", defaults.BoundaryAddr)
	v.SetDefault("oidc_auth_method_id", defaults.OIDCAuthMethodID)
	v.SetDefault("auth_token", defaults.AuthToken)
	v.SetDefault("default_project_scope_id", defaults.DefaultProjectScopeID)
	v.SetDefault("default_catalog_name", defaults.DefaultCatalogName)
	v.SetDefault("default_host_set_name", defaults.DefaultHostSetName)
	v.SetDefault("default_role_name", defaults.DefaultRoleName)
	v.SetDefault("default_principal_id", defaults.DefaultPrincipalID)
	v.SetDefault("default_target_port", defaults.DefaultTargetPort)
	v.SetDefault("default_create_role", defaults.DefaultCreateRole)
	v.SetDefault("default_connect_after_create", defaults.DefaultConnectAfterCreate)
	v.SetDefault("auto_select_default_scope", defaults.AutoSelectDefaultScope)
	v.SetDefault("last_host_catalog_id", defaults.LastHostCatalogID)
}

func applyConfig(v *viper.Viper, cfg Config) {
	v.Set("boundary_addr", cfg.BoundaryAddr)
	v.Set("oidc_auth_method_id", cfg.OIDCAuthMethodID)
	v.Set("auth_token", cfg.AuthToken)
	v.Set("default_project_scope_id", cfg.DefaultProjectScopeID)
	v.Set("default_catalog_name", cfg.DefaultCatalogName)
	v.Set("default_host_set_name", cfg.DefaultHostSetName)
	v.Set("default_role_name", cfg.DefaultRoleName)
	v.Set("default_principal_id", cfg.DefaultPrincipalID)
	v.Set("default_target_port", cfg.DefaultTargetPort)
	v.Set("default_create_role", cfg.DefaultCreateRole)
	v.Set("default_connect_after_create", cfg.DefaultConnectAfterCreate)
	v.Set("auto_select_default_scope", cfg.AutoSelectDefaultScope)
	v.Set("last_host_catalog_id", cfg.LastHostCatalogID)
}
