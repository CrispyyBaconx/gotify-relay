package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultListenAddr = ":8080"

type Config struct {
	Server  ServerConfig      `yaml:"server"`
	Gotify  GotifyConfig      `yaml:"gotify"`
	Callers map[string]Caller `yaml:"callers"`
	Groups  map[string]Group  `yaml:"groups"`
}

type ServerConfig struct {
	ListenAddr string `yaml:"listen_addr"`
}

type GotifyConfig struct {
	URL string `yaml:"url"`
}

type Caller struct {
	Token  string   `yaml:"token"`
	Groups []string `yaml:"groups"`
}

type Group struct {
	Targets []Target `yaml:"targets"`
}

type Target struct {
	Name     string `yaml:"name"`
	AppToken string `yaml:"app_token"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	expanded := os.ExpandEnv(string(data))
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (cfg *Config) Validate() error {
	if cfg.Server.ListenAddr == "" {
		cfg.Server.ListenAddr = DefaultListenAddr
	}
	if strings.TrimSpace(cfg.Gotify.URL) == "" {
		return errors.New("gotify.url is required")
	}
	if len(cfg.Callers) == 0 {
		return errors.New("at least one caller is required")
	}
	if len(cfg.Groups) == 0 {
		return errors.New("at least one group is required")
	}

	for groupName, group := range cfg.Groups {
		if strings.TrimSpace(groupName) == "" {
			return errors.New("group name is required")
		}
		if len(group.Targets) == 0 {
			return fmt.Errorf("group %q must have at least one target", groupName)
		}

		seenTargets := map[string]struct{}{}
		for i, target := range group.Targets {
			if strings.TrimSpace(target.Name) == "" {
				return fmt.Errorf("group %q target %d name is required", groupName, i)
			}
			if strings.TrimSpace(target.AppToken) == "" {
				return fmt.Errorf("group %q target %q app_token is required", groupName, target.Name)
			}
			if _, ok := seenTargets[target.Name]; ok {
				return fmt.Errorf("group %q has duplicate target %q", groupName, target.Name)
			}
			seenTargets[target.Name] = struct{}{}
		}
	}

	seenTokens := map[string]string{}
	for callerName, caller := range cfg.Callers {
		if strings.TrimSpace(callerName) == "" {
			return errors.New("caller name is required")
		}
		if strings.TrimSpace(caller.Token) == "" {
			return fmt.Errorf("caller %q token is required", callerName)
		}
		if existing, ok := seenTokens[caller.Token]; ok {
			return fmt.Errorf("caller %q token duplicates caller %q", callerName, existing)
		}
		seenTokens[caller.Token] = callerName
		if len(caller.Groups) == 0 {
			return fmt.Errorf("caller %q must allow at least one group", callerName)
		}
		for _, groupName := range caller.Groups {
			if _, ok := cfg.Groups[groupName]; !ok {
				return fmt.Errorf("caller %q references unknown group %q", callerName, groupName)
			}
		}
	}

	return nil
}
