package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultListenAddr = ":8080"
const DefaultSubscriptionsPath = "/data/subscriptions.json"

type Config struct {
	Server        ServerConfig       `yaml:"server"`
	Gotify        GotifyConfig       `yaml:"gotify"`
	Subscriptions SubscriptionConfig `yaml:"subscriptions"`
	Members       map[string]Member  `yaml:"members"`
	Channels      map[string]Channel `yaml:"channels"`
	Callers       map[string]Caller  `yaml:"callers"`
}

type ServerConfig struct {
	ListenAddr string `yaml:"listen_addr"`
}

type GotifyConfig struct {
	URL string `yaml:"url"`
}

type Caller struct {
	Token    string   `yaml:"token"`
	Channels []string `yaml:"channels"`
}

type SubscriptionConfig struct {
	Path string `yaml:"path"`
}

type Member struct {
	Token    string `yaml:"token"`
	AppToken string `yaml:"app_token"`
}

type Channel struct {
	DefaultSubscribed bool `yaml:"default_subscribed"`
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
	if cfg.Subscriptions.Path == "" {
		cfg.Subscriptions.Path = DefaultSubscriptionsPath
	}
	if strings.TrimSpace(cfg.Gotify.URL) == "" {
		return errors.New("gotify.url is required")
	}
	if len(cfg.Members) == 0 {
		return errors.New("at least one member is required")
	}
	if len(cfg.Channels) == 0 {
		return errors.New("at least one channel is required")
	}
	if len(cfg.Callers) == 0 {
		return errors.New("at least one caller is required")
	}

	seenTokens := map[string]string{}
	for memberName, member := range cfg.Members {
		if strings.TrimSpace(memberName) == "" {
			return errors.New("member name is required")
		}
		if strings.TrimSpace(member.Token) == "" {
			return fmt.Errorf("member %q token is required", memberName)
		}
		if strings.TrimSpace(member.AppToken) == "" {
			return fmt.Errorf("member %q app_token is required", memberName)
		}
		if existing, ok := seenTokens[member.Token]; ok {
			return fmt.Errorf("member %q token duplicates %s", memberName, existing)
		}
		seenTokens[member.Token] = "member " + memberName
	}

	for channelName := range cfg.Channels {
		if strings.TrimSpace(channelName) == "" {
			return errors.New("channel name is required")
		}
	}

	for callerName, caller := range cfg.Callers {
		if strings.TrimSpace(callerName) == "" {
			return errors.New("caller name is required")
		}
		if strings.TrimSpace(caller.Token) == "" {
			return fmt.Errorf("caller %q token is required", callerName)
		}
		if existing, ok := seenTokens[caller.Token]; ok {
			return fmt.Errorf("caller %q token duplicates %s", callerName, existing)
		}
		seenTokens[caller.Token] = "caller " + callerName
		if len(caller.Channels) == 0 {
			return fmt.Errorf("caller %q must allow at least one channel", callerName)
		}
		for _, channelName := range caller.Channels {
			if _, ok := cfg.Channels[channelName]; !ok {
				return fmt.Errorf("caller %q references unknown channel %q", callerName, channelName)
			}
		}
	}

	return nil
}
