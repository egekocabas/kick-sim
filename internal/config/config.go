package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

const FormatVersion = 1

type Config struct {
	Version            int                    `yaml:"version" json:"version"`
	DefaultDestination string                 `yaml:"defaultDestination" json:"defaultDestination"`
	Destinations       map[string]Destination `yaml:"destinations" json:"destinations"`
	Signing            Signing                `yaml:"signing" json:"signing"`
	Defaults           Defaults               `yaml:"defaults" json:"defaults"`
	Safety             Safety                 `yaml:"safety" json:"safety"`
	History            History                `yaml:"history" json:"history"`
}

type Destination struct {
	Class     string `yaml:"class" json:"class"`
	URL       string `yaml:"url" json:"url"`
	Timeout   string `yaml:"timeout" json:"timeout"`
	Redirects string `yaml:"redirects" json:"redirects"`
}

type Signing struct {
	Mode       string `yaml:"mode" json:"mode"`
	PrivateKey string `yaml:"privateKey" json:"privateKey"`
	PublicKey  string `yaml:"publicKey" json:"publicKey"`
}

type Defaults struct {
	Broadcaster User `yaml:"broadcaster" json:"broadcaster"`
	Sender      User `yaml:"sender" json:"sender"`
}

type User struct {
	UserID         int64  `yaml:"user_id" json:"user_id"`
	Username       string `yaml:"username" json:"username"`
	ChannelSlug    string `yaml:"channel_slug" json:"channel_slug"`
	IsVerified     bool   `yaml:"is_verified" json:"is_verified"`
	ProfilePicture string `yaml:"profile_picture,omitempty" json:"profile_picture,omitempty"`
}

type Safety struct {
	AllowedDestinationClasses []string `yaml:"allowedDestinationClasses" json:"allowedDestinationClasses"`
}

type History struct {
	Enabled              bool   `yaml:"enabled" json:"enabled"`
	MaxAge               string `yaml:"maxAge" json:"maxAge"`
	MaxFunctionalRuns    int    `yaml:"maxFunctionalRuns" json:"maxFunctionalRuns"`
	StoreResponseBodies  bool   `yaml:"storeResponseBodies" json:"storeResponseBodies"`
	MaxResponseBodyBytes int    `yaml:"maxResponseBodyBytes" json:"maxResponseBodyBytes"`
}

func Default() Config {
	return Config{
		Version:            FormatVersion,
		DefaultDestination: "local",
		Destinations: map[string]Destination{
			"local": {
				Class:     "loopback",
				URL:       "http://127.0.0.1:3000/webhooks/kick",
				Timeout:   "5s",
				Redirects: "deny",
			},
		},
		Signing: Signing{
			Mode:       "simulator",
			PrivateKey: "./keys/private-key.pem",
			PublicKey:  "./keys/public-key.pem",
		},
		Defaults: Defaults{
			Broadcaster: User{
				UserID:         123456789,
				Username:       "test_streamer",
				ChannelSlug:    "test-streamer",
				IsVerified:     false,
				ProfilePicture: "https://example.invalid/kick-sim/test_streamer.png",
			},
			Sender: User{
				UserID:         987654321,
				Username:       "test_viewer",
				ChannelSlug:    "test-viewer",
				IsVerified:     false,
				ProfilePicture: "https://example.invalid/kick-sim/test_viewer.png",
			},
		},
		Safety: Safety{AllowedDestinationClasses: []string{"loopback"}},
		History: History{
			Enabled:              true,
			MaxAge:               "720h",
			MaxFunctionalRuns:    1000,
			StoreResponseBodies:  true,
			MaxResponseBodyBytes: 64 * 1024,
		},
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read configuration: %w", err)
	}
	value := Config{History: Default().History}
	if err := yaml.UnmarshalWithOptions(data, &value, yaml.Strict()); err != nil {
		return Config{}, fmt.Errorf("parse configuration: %w", err)
	}
	if err := Validate(value); err != nil {
		return Config{}, err
	}
	return value, nil
}

func Marshal(value Config) ([]byte, error) {
	data, err := yaml.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal configuration: %w", err)
	}
	return data, nil
}

func Validate(value Config) error {
	var problems []error
	if value.Version != FormatVersion {
		problems = append(problems, fmt.Errorf("unsupported config version %d", value.Version))
	}
	if value.DefaultDestination == "" {
		problems = append(problems, errors.New("defaultDestination is required"))
	} else if _, ok := value.Destinations[value.DefaultDestination]; !ok {
		problems = append(problems, fmt.Errorf("default destination %q does not exist", value.DefaultDestination))
	}
	if value.Signing.Mode != "simulator" {
		problems = append(problems, errors.New("signing.mode must be simulator"))
	}
	if value.Signing.PrivateKey == "" || value.Signing.PublicKey == "" {
		problems = append(problems, errors.New("signing key paths are required"))
	}

	names := make([]string, 0, len(value.Destinations))
	for name := range value.Destinations {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		destination := value.Destinations[name]
		if destination.Class != "loopback" {
			problems = append(problems, fmt.Errorf("destination %q must use class loopback", name))
		}
		if destination.Redirects != "deny" {
			problems = append(problems, fmt.Errorf("destination %q must deny redirects", name))
		}
		if _, err := time.ParseDuration(destination.Timeout); err != nil {
			problems = append(problems, fmt.Errorf("destination %q has invalid timeout: %w", name, err))
		}
		if err := validateLoopbackURL(destination.URL); err != nil {
			problems = append(problems, fmt.Errorf("destination %q: %w", name, err))
		}
	}
	if len(value.Safety.AllowedDestinationClasses) != 1 || value.Safety.AllowedDestinationClasses[0] != "loopback" {
		problems = append(problems, errors.New("allowedDestinationClasses must contain only loopback"))
	}
	if _, err := time.ParseDuration(value.History.MaxAge); err != nil {
		problems = append(problems, fmt.Errorf("history.maxAge is invalid: %w", err))
	}
	if value.History.MaxFunctionalRuns < 1 {
		problems = append(problems, errors.New("history.maxFunctionalRuns must be positive"))
	}
	if value.History.MaxResponseBodyBytes < 0 || value.History.MaxResponseBodyBytes > 1024*1024 {
		problems = append(problems, errors.New("history.maxResponseBodyBytes must be between 0 and 1048576"))
	}
	for name, user := range map[string]User{"broadcaster": value.Defaults.Broadcaster, "sender": value.Defaults.Sender} {
		if user.UserID <= 0 || user.Username == "" || user.ChannelSlug == "" {
			problems = append(problems, fmt.Errorf("defaults.%s requires a positive user_id, username, and channel_slug", name))
		}
	}
	return errors.Join(problems...)
}

func ResolvePath(workspaceRoot, configuredPath string) string {
	if filepath.IsAbs(configuredPath) {
		return filepath.Clean(configuredPath)
	}
	return filepath.Clean(filepath.Join(workspaceRoot, configuredPath))
}

func (value Config) Destination(name string) (Destination, error) {
	if name == "" {
		name = value.DefaultDestination
	}
	destination, ok := value.Destinations[name]
	if !ok {
		return Destination{}, fmt.Errorf("destination %q does not exist", name)
	}
	return destination, nil
}

func validateLoopbackURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("URL must use http or https")
	}
	if parsed.User != nil || parsed.Hostname() == "" {
		return errors.New("URL must contain a host and no user information")
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("URL must use a loopback host")
	}
	return nil
}
