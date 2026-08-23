package compatibility

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/egekocabas/kick-sim/assets"
)

type Metadata struct {
	Upstream         Upstream         `json:"upstream"`
	SupportedEvents  map[string][]int `json:"supportedEvents"`
	KnownDifferences []string         `json:"knownDifferences"`
	BundleDigest     string           `json:"eventSchemaBundleDigest"`
}

type Upstream struct {
	Repository  string   `json:"repository"`
	Commit      string   `json:"commit"`
	RetrievedAt string   `json:"retrievedAt"`
	Documents   []string `json:"documents"`
}

func Load() (Metadata, error) {
	data, err := assets.Files.ReadFile("compatibility/metadata.json")
	if err != nil {
		return Metadata{}, fmt.Errorf("read compatibility metadata: %w", err)
	}
	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return Metadata{}, fmt.Errorf("parse compatibility metadata: %w", err)
	}

	hash := sha256.New()
	for _, path := range []string{
		"events/chat.message.sent.v1.schema.json",
		"events/chat.message.sent.v1.defaults.json",
	} {
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
