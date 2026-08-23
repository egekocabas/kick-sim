package simulator

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egekocabas/kick-sim/internal/delivery"
	"github.com/egekocabas/kick-sim/internal/signing"
)

func TestTriggerChatMessageDeliversVerifiableRequest(t *testing.T) {
	t.Parallel()

	privateKey, err := signing.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	fixedTime := time.Date(2026, time.August, 24, 10, 15, 30, 123000000, time.UTC)
	ids := []string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAV",
		"01ARZ3NDEKTSV4RRFFQ69G5FAW",
		"01ARZ3NDEKTSV4RRFFQ69G5FAX",
	}
	nextID := func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	}

	receiverErrors := make(chan error, 1)
	reportReceiverError := func(err error) {
		select {
		case receiverErrors <- err:
		default:
		}
	}
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			reportReceiverError(readErr)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		checks := map[string]string{
			delivery.HeaderMessageID:      "01ARZ3NDEKTSV4RRFFQ69G5FAW",
			delivery.HeaderSubscriptionID: "01ARZ3NDEKTSV4RRFFQ69G5FAX",
			delivery.HeaderTimestamp:      "2026-08-24T10:15:30.123Z",
			delivery.HeaderEventType:      ChatMessageSentType,
			delivery.HeaderEventVersion:   "1",
		}
		for header, want := range checks {
			if got := request.Header.Get(header); got != want {
				reportReceiverError(fmt.Errorf("header %s = %q, want %q", header, got, want))
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		if err := signing.Verify(
			&privateKey.PublicKey,
			request.Header.Get(delivery.HeaderMessageID),
			request.Header.Get(delivery.HeaderTimestamp),
			body,
			request.Header.Get(delivery.HeaderSignature),
		); err != nil {
			reportReceiverError(err)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writer.WriteHeader(http.StatusAccepted)
	})
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	receiver := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: handler},
	}
	receiver.Start()
	defer receiver.Close()

	result, err := TriggerChatMessage(context.Background(), privateKey, ChatMessageOptions{
		DestinationURL: receiver.URL + "/webhooks/kick",
		Content:        "Hello from a deterministic test",
		Sender:         UserInput{UserID: 42, Username: "viewer_42"},
		Broadcaster:    UserInput{UserID: 7, Username: "creator", IsVerified: true},
		Now:            func() time.Time { return fixedTime },
		NewID:          nextID,
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case receiverError := <-receiverErrors:
		t.Fatal(receiverError)
	default:
	}
	if result.StatusCode != http.StatusAccepted {
		t.Fatalf("StatusCode = %d, want %d", result.StatusCode, http.StatusAccepted)
	}
	if result.MessageID != "01ARZ3NDEKTSV4RRFFQ69G5FAW" {
		t.Fatalf("MessageID = %q", result.MessageID)
	}
	if result.SubscriptionID != "01ARZ3NDEKTSV4RRFFQ69G5FAX" {
		t.Fatalf("SubscriptionID = %q", result.SubscriptionID)
	}

	wantBody := `{"message_id":"01ARZ3NDEKTSV4RRFFQ69G5FAV","replies_to":null,"broadcaster":{"is_anonymous":false,"user_id":7,"username":"creator","is_verified":true,"profile_picture":"https://example.com/creator.jpg","channel_slug":"creator","identity":null},"sender":{"is_anonymous":false,"user_id":42,"username":"viewer_42","is_verified":false,"profile_picture":"https://example.com/viewer_42.jpg","channel_slug":"viewer_42","identity":{"username_color":"#FFFFFF","badges":[]}},"content":"Hello from a deterministic test","emotes":[],"created_at":"2026-08-24T10:15:30.123Z"}`
	if got := string(result.RawBody); got != wantBody {
		t.Fatalf("RawBody mismatch\n got: %s\nwant: %s", got, wantBody)
	}
	if strings.Contains(string(result.RawBody), "\n") {
		t.Fatal("RawBody unexpectedly contains formatting whitespace")
	}
}
