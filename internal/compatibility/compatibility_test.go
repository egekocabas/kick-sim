package compatibility

import (
	"reflect"
	"strings"
	"testing"

	"github.com/egekocabas/kick-sim/internal/events"
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
	const expectedDigest = "sha256:c7956d5d43812ce7557902419718f4013781c0b6071e0eae1913abda3a3230bf"
	if metadata.BundleDigest != expectedDigest {
		t.Fatalf("bundle digest = %q, want %q", metadata.BundleDigest, expectedDigest)
	}

	registry, err := events.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	registered := make(map[string][]int)
	for _, definition := range registry.List() {
		registered[definition.Type] = append(registered[definition.Type], definition.Version)
	}
	if !reflect.DeepEqual(metadata.SupportedEvents, registered) {
		t.Fatalf("compatibility metadata = %#v, registry = %#v", metadata.SupportedEvents, registered)
	}
}

func TestBundlePathsAreDeterministicAndComplete(t *testing.T) {
	t.Parallel()

	got := bundlePaths(map[string][]int{
		"z.event": {2, 1},
		"a.event": {1},
	})
	want := []string{
		"events/a.event.v1.schema.json",
		"events/a.event.v1.defaults.json",
		"events/z.event.v1.schema.json",
		"events/z.event.v1.defaults.json",
		"events/z.event.v2.schema.json",
		"events/z.event.v2.defaults.json",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bundle paths = %#v, want %#v", got, want)
	}
}
