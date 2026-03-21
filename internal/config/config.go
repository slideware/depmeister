package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type AllowlistEntry struct {
	Name   string `yaml:"name"`
	Reason string `yaml:"reason"`
	Vulns  bool   `yaml:"vulns"`
	Age    bool   `yaml:"age"`
	Score  bool   `yaml:"score"`
}

type Config struct {
	MaxAgeDays   int              `yaml:"max_age_days"`
	MinScorecard float64          `yaml:"min_scorecard"`
	FailOnVulns  bool             `yaml:"fail_on_vulns"`
	SkipIndirect bool             `yaml:"skip_indirect"`
	SkipDevDeps  bool             `yaml:"skip_dev_deps"`
	Concurrency  int              `yaml:"concurrency"`
	Recursive    bool             `yaml:"-"`
	Dir          string           `yaml:"-"`
	Format       string           `yaml:"-"`
	ConfigPath   string           `yaml:"-"`
	Verbose      bool             `yaml:"-"`
	Allowlist    []AllowlistEntry `yaml:"allowlist"`
}

func Default() Config {
	return Config{
		MaxAgeDays:   7,
		MinScorecard: 2.0,
		FailOnVulns:  true,
		SkipIndirect: true,
		SkipDevDeps:  false,
		Concurrency:  10,
		Dir:          ".",
		Format:       "text",
		ConfigPath:   ".depmeister.yaml",
		Verbose:      false,
	}
}

// Load reads config from the YAML file at cfg.ConfigPath (if it exists),
// then applies environment variable overrides. The provided cfg should
// already have defaults and any flag overrides applied.
func Load(cfg Config) (Config, error) {
	// Read YAML file if it exists.
	yamlCfg, yamlLoaded, err := loadYAML(cfg.ConfigPath)
	if err != nil {
		return cfg, fmt.Errorf("reading config file: %w", err)
	}

	if yamlLoaded {
		mergeYAML(&cfg, yamlCfg)
	}

	applyEnv(&cfg)

	if cfg.Concurrency < 1 {
		cfg.Concurrency = 1
	}

	return cfg, nil
}

func loadYAML(path string) (Config, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, false, nil
		}
		return Config{}, false, err
	}

	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, false, fmt.Errorf("parsing YAML: %w", err)
	}
	return c, true, nil
}

// mergeYAML applies YAML values onto cfg. Fields that are zero-valued in
// yamlCfg are left untouched (keeping the default).
func mergeYAML(cfg *Config, yamlCfg Config) {
	if yamlCfg.MaxAgeDays != 0 {
		cfg.MaxAgeDays = yamlCfg.MaxAgeDays
	}
	if yamlCfg.MinScorecard != 0 {
		cfg.MinScorecard = yamlCfg.MinScorecard
	}
	// FailOnVulns: YAML can set it to false, but since the zero value of
	// bool is false we can't distinguish "not set" from "set to false".
	// We always apply the YAML value for booleans.
	cfg.FailOnVulns = yamlCfg.FailOnVulns
	cfg.SkipIndirect = yamlCfg.SkipIndirect
	cfg.SkipDevDeps = yamlCfg.SkipDevDeps

	if yamlCfg.Concurrency != 0 {
		cfg.Concurrency = yamlCfg.Concurrency
	}
	if len(yamlCfg.Allowlist) > 0 {
		cfg.Allowlist = yamlCfg.Allowlist
	}
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("DEPMEISTER_MAX_AGE_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxAgeDays = n
		}
	}
	if v := os.Getenv("DEPMEISTER_MIN_SCORECARD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.MinScorecard = f
		}
	}
	if v := os.Getenv("DEPMEISTER_FAIL_ON_VULNS"); v != "" {
		cfg.FailOnVulns = parseBool(v)
	}
	if v := os.Getenv("DEPMEISTER_SKIP_INDIRECT"); v != "" {
		cfg.SkipIndirect = parseBool(v)
	}
	if v := os.Getenv("DEPMEISTER_SKIP_DEV_DEPS"); v != "" {
		cfg.SkipDevDeps = parseBool(v)
	}
	if v := os.Getenv("DEPMEISTER_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Concurrency = n
		}
	}
	if v := os.Getenv("DEPMEISTER_RECURSIVE"); v != "" {
		cfg.Recursive = parseBool(v)
	}
	if v := os.Getenv("DEPMEISTER_DIR"); v != "" {
		cfg.Dir = v
	}
	if v := os.Getenv("DEPMEISTER_FORMAT"); v != "" {
		cfg.Format = strings.ToLower(v)
	}
	if v := os.Getenv("DEPMEISTER_VERBOSE"); v != "" {
		cfg.Verbose = parseBool(v)
	}
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "1" || s == "true" || s == "yes"
}
