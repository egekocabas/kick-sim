// Package loopback defines the URL policy shared by configuration and delivery.
package loopback

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateURL rejects destinations that are not explicit HTTP(S) loopback URLs.
// Runtime delivery must still validate DNS results because localhost mappings can
// change after configuration is loaded.
func ValidateURL(rawURL string) error {
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
	if parsed.Fragment != "" {
		return errors.New("URL must not contain a fragment")
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
