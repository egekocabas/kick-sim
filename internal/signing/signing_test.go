package signing

import (
	"bytes"
	"testing"
)

func TestSignatureInput(t *testing.T) {
	t.Parallel()

	got := SignatureInput(
		"01KICKMESSAGE00000000000000",
		"2026-08-24T10:15:30Z",
		[]byte(`{"content":"hello"}`),
	)
	want := []byte(`01KICKMESSAGE00000000000000.2026-08-24T10:15:30Z.{"content":"hello"}`)
	if !bytes.Equal(got, want) {
		t.Fatalf("SignatureInput() = %q, want %q", got, want)
	}
}

func TestSignAndVerifyExactBody(t *testing.T) {
	t.Parallel()

	privateKey, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"content":"hello"}`)
	signature, err := Sign(privateKey, "message-id", "2026-08-24T10:15:30Z", body)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(&privateKey.PublicKey, "message-id", "2026-08-24T10:15:30Z", body, signature); err != nil {
		t.Fatalf("Verify() rejected the signed body: %v", err)
	}

	mutated := append([]byte(nil), body...)
	mutated[len(mutated)-2] = 'H'
	if err := Verify(&privateKey.PublicKey, "message-id", "2026-08-24T10:15:30Z", mutated, signature); err == nil {
		t.Fatal("Verify() accepted a mutated body")
	}
}

func TestKeyPEMRoundTrip(t *testing.T) {
	t.Parallel()

	privateKey, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	privatePEM, err := MarshalPrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	parsedPrivate, err := ParsePrivateKey(privatePEM)
	if err != nil {
		t.Fatal(err)
	}
	publicPEM, err := MarshalPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	parsedPublic, err := ParsePublicKey(publicPEM)
	if err != nil {
		t.Fatal(err)
	}
	if parsedPrivate.N.Cmp(privateKey.N) != 0 || parsedPublic.N.Cmp(privateKey.N) != 0 {
		t.Fatal("parsed key material does not match the generated key")
	}
}
