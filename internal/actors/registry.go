package actors

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"

	"github.com/goccy/go-yaml"
)

// FormatVersion is the only actor-registry version understood by this release.
const FormatVersion = 1

var actorID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// User contains the event payload fields owned by a reusable actor.
type User struct {
	UserID         int64  `yaml:"user_id" json:"user_id"`
	Username       string `yaml:"username" json:"username"`
	ChannelSlug    string `yaml:"channel_slug" json:"channel_slug"`
	IsVerified     bool   `yaml:"is_verified" json:"is_verified"`
	ProfilePicture string `yaml:"profile_picture,omitempty" json:"profile_picture,omitempty"`
}

// File is the versioned on-disk representation of an actor registry.
type File struct {
	Version int             `yaml:"version"`
	Users   map[string]User `yaml:"users"`
}

// Registry stores validated actors by their scenario-facing identifier.
type Registry struct {
	users map[string]User
}

// Empty returns a registry with no actors.
func Empty() *Registry { return &Registry{users: map[string]User{}} }

// Load strictly decodes and validates an actor registry, treating a missing file as empty.
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Empty(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read actor registry: %w", err)
	}
	var document File
	if err := yaml.UnmarshalWithOptions(data, &document, yaml.Strict()); err != nil {
		return nil, fmt.Errorf("parse actor registry: %w", err)
	}
	if document.Version != FormatVersion {
		return nil, fmt.Errorf("unsupported actor registry version %d", document.Version)
	}
	seenUserIDs := map[int64]string{}
	for id, user := range document.Users {
		if !actorID.MatchString(id) {
			return nil, fmt.Errorf("invalid actor ID %q", id)
		}
		if user.UserID < 1 {
			return nil, fmt.Errorf("actor %q user_id must be a positive integer", id)
		}
		if prior, exists := seenUserIDs[user.UserID]; exists {
			return nil, fmt.Errorf("actors %q and %q use the same user_id %d", prior, id, user.UserID)
		}
		seenUserIDs[user.UserID] = id
		if user.Username == "" || user.ChannelSlug == "" {
			return nil, fmt.Errorf("actor %q requires username and channel_slug", id)
		}
	}
	return &Registry{users: document.Users}, nil
}

// Get returns the actor registered under id.
func (registry *Registry) Get(id string) (User, error) {
	if registry == nil {
		return User{}, fmt.Errorf("actor %q was not found", id)
	}
	user, ok := registry.users[id]
	if !ok {
		return User{}, fmt.Errorf("actor %q was not found", id)
	}
	return user, nil
}

// List returns a copy of all registered actors.
func (registry *Registry) List() map[string]User {
	result := make(map[string]User, len(registry.users))
	for id, user := range registry.users {
		result[id] = user
	}
	return result
}

// IDs returns all actor identifiers in lexical order.
func (registry *Registry) IDs() []string {
	ids := make([]string, 0, len(registry.users))
	for id := range registry.users {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
