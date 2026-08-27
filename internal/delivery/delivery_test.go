package delivery

import (
	"context"
	"testing"

	"github.com/egekocabas/kick-sim/internal/loopback"
)

func TestSendRejectsNonLoopbackDestination(t *testing.T) {
	t.Parallel()

	_, err := Send(context.Background(), nil, "https://example.com/webhooks/kick", Metadata{}, []byte(`{}`))
	if err == nil {
		t.Fatal("Send() accepted a non-loopback destination")
	}
}

func TestValidateLoopbackDestination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		destination string
		wantError   bool
	}{
		{name: "IPv4", destination: "http://127.0.0.1:3000/webhooks/kick"},
		{name: "IPv6", destination: "http://[::1]:3000/webhooks/kick"},
		{name: "localhost", destination: "http://localhost:3000/webhooks/kick"},
		{name: "localhost trailing dot", destination: "http://localhost.:3000/webhooks/kick"},
		{name: "remote hostname", destination: "https://example.com/webhooks/kick", wantError: true},
		{name: "remote IP", destination: "http://192.0.2.1/webhooks/kick", wantError: true},
		{name: "credentials", destination: "http://user@localhost/webhooks/kick", wantError: true},
		{name: "fragment", destination: "http://localhost/webhooks/kick#ignored", wantError: true},
		{name: "unsupported scheme", destination: "ftp://localhost/webhooks/kick", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := loopback.ValidateURL(test.destination)
			if test.wantError && err == nil {
				t.Fatalf("validateLoopbackDestination(%q) returned nil", test.destination)
			}
			if !test.wantError && err != nil {
				t.Fatalf("validateLoopbackDestination(%q) returned %v", test.destination, err)
			}
		})
	}
}
