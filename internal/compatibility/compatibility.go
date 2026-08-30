package compatibility

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/egekocabas/kick-sim/assets"
)

// Metadata records upstream provenance and the supported event-contract surface.
type Metadata struct {
	Upstream         Upstream         `json:"upstream"`
	SupportedEvents  map[string][]int `json:"supportedEvents"`
	KnownDifferences []string         `json:"knownDifferences"`
	BundleDigest     string           `json:"eventSchemaBundleDigest"`
}

// Upstream identifies the source material used for compatibility research.
type Upstream struct {
	Repository  string   `json:"repository"`
	Commit      string   `json:"commit"`
	RetrievedAt string   `json:"retrievedAt"`
	Documents   []string `json:"documents"`
}

// Load validates bundled compatibility metadata and computes its schema-bundle digest.
func Load() (Metadata, error) {
	data, err := assets.Files.ReadFile("compatibility/metadata.json")
	if err != nil {
		return Metadata{}, fmt.Errorf("read compatibility metadata: %w", err)
	}
	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return Metadata{}, fmt.Errorf("parse compatibility metadata: %w", err)
	}
	if err := validateMetadata(metadata); err != nil {
		return Metadata{}, err
	}

	hash := sha256.New()
	for _, path := range bundlePaths(metadata.SupportedEvents) {
		content, err := assets.Files.ReadFile(path)
		if err != nil {
			return Metadata{}, fmt.Errorf("read compatibility asset %s: %w", path, err)
		}
		_, _ = hash.Write([]byte(path))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(content)
		_, _ = hash.Write([]byte{0})
	}
	metadata.BundleDigest = "sha256:" + hex.EncodeToString(hash.Sum(nil))
	return metadata, nil
}

func validateMetadata(metadata Metadata) error {
	if metadata.Upstream.Repository == "" {
		return fmt.Errorf("compatibility metadata upstream repository is required")
	}
	if len(metadata.Upstream.Commit) != 40 || strings.ToLower(metadata.Upstream.Commit) != metadata.Upstream.Commit {
		return fmt.Errorf("compatibility metadata upstream commit must be a lowercase 40-character SHA")
	}
	if _, err := hex.DecodeString(metadata.Upstream.Commit); err != nil {
		return fmt.Errorf("compatibility metadata upstream commit: %w", err)
	}
	if _, err := time.Parse("2006-01-02", metadata.Upstream.RetrievedAt); err != nil {
		return fmt.Errorf("compatibility metadata retrievedAt: %w", err)
	}
	if len(metadata.Upstream.Documents) == 0 {
		return fmt.Errorf("compatibility metadata upstream documents are required")
	}
	if len(metadata.SupportedEvents) == 0 {
		return fmt.Errorf("compatibility metadata supported events are required")
	}
	for eventType, versions := range metadata.SupportedEvents {
		if eventType == "" || len(versions) == 0 {
			return fmt.Errorf("compatibility metadata event type and versions are required")
		}
		seen := make(map[int]struct{}, len(versions))
		for _, version := range versions {
			if version < 1 {
				return fmt.Errorf("compatibility metadata %s version must be positive", eventType)
			}
			if _, exists := seen[version]; exists {
				return fmt.Errorf("compatibility metadata %s has duplicate version %d", eventType, version)
			}
			seen[version] = struct{}{}
		}
	}
	return nil
}

func bundlePaths(supported map[string][]int) []string {
	eventTypes := make([]string, 0, len(supported))
	for eventType := range supported {
		eventTypes = append(eventTypes, eventType)
	}
	sort.Strings(eventTypes)

	paths := make([]string, 0, len(eventTypes)*2)
	for _, eventType := range eventTypes {
		versions := append([]int(nil), supported[eventType]...)
		sort.Ints(versions)
		for _, version := range versions {
			base := fmt.Sprintf("events/%s.v%d", eventType, version)
			paths = append(paths, base+".schema.json", base+".defaults.json")
		}
	}
	return paths
}
