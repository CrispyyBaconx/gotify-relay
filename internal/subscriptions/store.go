package subscriptions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"gotify-relay/internal/config"
)

type Subscription struct {
	Channel    string `json:"channel"`
	Subscribed bool   `json:"subscribed"`
}

type JSONStore struct {
	path     string
	members  map[string]struct{}
	channels map[string]config.Channel
	mu       sync.Mutex
	state    stateFile
}

type stateFile struct {
	Subscriptions map[string]map[string]bool `json:"subscriptions"`
}

func NewJSONStore(path string, cfg *config.Config) (*JSONStore, error) {
	if path == "" {
		return nil, errors.New("subscription store path is required")
	}

	store := &JSONStore{
		path:     path,
		members:  map[string]struct{}{},
		channels: map[string]config.Channel{},
		state: stateFile{
			Subscriptions: map[string]map[string]bool{},
		},
	}
	for member := range cfg.Members {
		store.members[member] = struct{}{}
	}
	for channel, channelConfig := range cfg.Channels {
		store.channels[channel] = channelConfig
	}

	if err := store.load(); err != nil {
		return nil, err
	}
	store.applyDefaults()

	return store, nil
}

func (s *JSONStore) List(member string) ([]Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireMember(member); err != nil {
		return nil, err
	}

	channels := sortedChannelNames(s.channels)
	items := make([]Subscription, 0, len(channels))
	for _, channel := range channels {
		items = append(items, Subscription{
			Channel:    channel,
			Subscribed: s.state.Subscriptions[member][channel],
		})
	}
	return items, nil
}

func (s *JSONStore) Set(member string, channel string, subscribed bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireMember(member); err != nil {
		return err
	}
	if err := s.requireChannel(channel); err != nil {
		return err
	}

	s.state.Subscriptions[member][channel] = subscribed
	return s.persist()
}

func (s *JSONStore) SubscribedMembers(channel string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireChannel(channel); err != nil {
		return nil, err
	}

	var members []string
	for _, member := range sortedMemberNames(s.members) {
		if s.state.Subscriptions[member][channel] {
			members = append(members, member)
		}
	}
	return members, nil
}

func (s *JSONStore) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read subscriptions: %w", err)
	}
	if err := json.Unmarshal(data, &s.state); err != nil {
		return fmt.Errorf("parse subscriptions: %w", err)
	}
	if s.state.Subscriptions == nil {
		s.state.Subscriptions = map[string]map[string]bool{}
	}
	return nil
}

func (s *JSONStore) applyDefaults() {
	for member := range s.members {
		if s.state.Subscriptions[member] == nil {
			s.state.Subscriptions[member] = map[string]bool{}
		}
		for channel, channelConfig := range s.channels {
			if _, ok := s.state.Subscriptions[member][channel]; !ok {
				s.state.Subscriptions[member][channel] = channelConfig.DefaultSubscribed
			}
		}
	}
}

func (s *JSONStore) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create subscription directory: %w", err)
	}

	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode subscriptions: %w", err)
	}
	data = append(data, '\n')

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write subscriptions: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace subscriptions: %w", err)
	}
	return nil
}

func (s *JSONStore) requireMember(member string) error {
	if _, ok := s.members[member]; !ok {
		return fmt.Errorf("unknown member %q", member)
	}
	return nil
}

func (s *JSONStore) requireChannel(channel string) error {
	if _, ok := s.channels[channel]; !ok {
		return fmt.Errorf("unknown channel %q", channel)
	}
	return nil
}

func sortedChannelNames(channels map[string]config.Channel) []string {
	names := make([]string, 0, len(channels))
	for name := range channels {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedMemberNames(members map[string]struct{}) []string {
	names := make([]string, 0, len(members))
	for name := range members {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
