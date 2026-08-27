package loopback

import "testing"

func TestValidateURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		url       string
		wantError bool
	}{
		{url: "http://127.0.0.1:3000/webhooks/kick"},
		{url: "http://[::1]:3000/webhooks/kick"},
		{url: "http://localhost.:3000/webhooks/kick"},
		{url: "https://example.com/webhooks/kick", wantError: true},
		{url: "http://user@localhost/webhooks/kick", wantError: true},
		{url: "http://localhost/webhooks/kick#fragment", wantError: true},
		{url: "ftp://localhost/webhooks/kick", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.url, func(t *testing.T) {
			t.Parallel()
			err := ValidateURL(test.url)
			if (err != nil) != test.wantError {
				t.Fatalf("ValidateURL(%q) error = %v", test.url, err)
			}
		})
	}
}
