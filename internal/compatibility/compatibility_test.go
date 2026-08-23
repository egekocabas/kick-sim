package compatibility

import (
	"strings"
	"testing"
)

func TestMetadataPinsFullUpstreamRevisionAndBundleDigest(t *testing.T) {
	t.Parallel()

	metadata, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata.Upstream.Commit) != 40 {
		t.Fatalf("upstream commit length = %d", len(metadata.Upstream.Commit))
	}
	if !strings.HasPrefix(metadata.BundleDigest, "sha256:") || len(metadata.BundleDigest) != len("sha256:")+64 {
		t.Fatalf("bundle digest = %q", metadata.BundleDigest)
	}
	if versions := metadata.SupportedEvents["chat.message.sent"]; len(versions) != 1 || versions[0] != 1 {
		t.Fatalf("supported events = %#v", metadata.SupportedEvents)
	}
}
