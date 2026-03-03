package config

import (
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	InboxAppID              string `yaml:"inbox_app_id"`
	InboxAppSecret          string `yaml:"inbox_app_secret"`
	InboxDefaultMailbox     int    `yaml:"inbox_default_mailbox,omitempty"`
	InboxPermissions        string `yaml:"inbox_permissions,omitempty"`
	InboxPIIMode            string `yaml:"inbox_pii_mode,omitempty"`
	InboxPIIAllowUnredacted bool   `yaml:"inbox_pii_allow_unredacted,omitempty"`
	DocsAPIKey              string `yaml:"docs_api_key,omitempty"`
	DocsPermissions         string `yaml:"docs_permissions,omitempty"`
	Format                  string `yaml:"format,omitempty"`
}

func DefaultPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "hs", "config.yaml")
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath()
	}
	cfg := &Config{Format: "table"}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			applyEnv(cfg)
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	applyEnv(cfg)
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("HS_INBOX_APP_ID"); v != "" {
		cfg.InboxAppID = v
	}
	if v := os.Getenv("HS_INBOX_APP_SECRET"); v != "" {
		cfg.InboxAppSecret = v
	}
	if v := os.Getenv("HS_FORMAT"); v != "" {
		cfg.Format = v
	}
	if v := os.Getenv("HS_INBOX_PERMISSIONS"); v != "" {
		cfg.InboxPermissions = v
	}
	if v := os.Getenv("HS_INBOX_PII_MODE"); v != "" {
		cfg.InboxPIIMode = v
	}
	if v := os.Getenv("HS_INBOX_PII_ALLOW_UNREDACTED"); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			cfg.InboxPIIAllowUnredacted = parsed
		}
	}
	if v := os.Getenv("HS_DOCS_API_KEY"); v != "" {
		cfg.DocsAPIKey = v
	}
	if v := os.Getenv("HS_DOCS_PERMISSIONS"); v != "" {
		cfg.DocsPermissions = v
	}
}

func Save(path string, cfg *Config) error {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
