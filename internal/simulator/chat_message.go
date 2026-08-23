package simulator

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"time"

	"github.com/egekocabas/kick-sim/internal/delivery"
	"github.com/egekocabas/kick-sim/internal/events"
	"github.com/egekocabas/kick-sim/internal/signing"
	"github.com/oklog/ulid/v2"
)

const ChatMessageSentType = "chat.message.sent"

type UserInput struct {
	UserID     int64
	Username   string
	IsVerified bool
}

type ChatMessageOptions struct {
	DestinationURL string
	Content        string
	Sender         UserInput
	Broadcaster    UserInput
	Timeout        time.Duration

	Now   func() time.Time
	NewID func() string
}

type ChatMessageResult struct {
	delivery.Result
	MessageID      string
	SubscriptionID string
	RawBody        []byte
}

func TriggerChatMessage(ctx context.Context, privateKey *rsa.PrivateKey, options ChatMessageOptions) (ChatMessageResult, error) {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	newID := options.NewID
	if newID == nil {
		newID = func() string { return ulid.Make().String() }
	}

	timestamp := now().UTC()
	payload := events.NewChatMessageSent(events.ChatMessageInput{
		MessageID:   newID(),
		Content:     options.Content,
		CreatedAt:   timestamp,
		Sender:      eventUser(options.Sender, true),
		Broadcaster: eventUser(options.Broadcaster, false),
	})
	body, err := json.Marshal(payload)
	if err != nil {
		return ChatMessageResult{}, fmt.Errorf("serialize chat event: %w", err)
	}

	messageID := newID()
	subscriptionID := newID()
	formattedTimestamp := timestamp.Format(time.RFC3339Nano)
	signature, err := signing.Sign(privateKey, messageID, formattedTimestamp, body)
	if err != nil {
		return ChatMessageResult{}, err
	}

	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := delivery.NewLoopbackClient(timeout)
	deliveryResult, err := delivery.Send(ctx, client, options.DestinationURL, delivery.Metadata{
		MessageID:      messageID,
		SubscriptionID: subscriptionID,
		Signature:      signature,
		Timestamp:      formattedTimestamp,
		EventType:      ChatMessageSentType,
		EventVersion:   "1",
	}, body)
	result := ChatMessageResult{
		Result:         deliveryResult,
		MessageID:      messageID,
		SubscriptionID: subscriptionID,
		RawBody:        body,
	}
	return result, err
}

func eventUser(input UserInput, includeIdentity bool) events.User {
	var identity *events.Identity
	if includeIdentity {
		identity = &events.Identity{
			UsernameColor: "#FFFFFF",
			Badges:        []events.Badge{},
		}
	}

	return events.User{
		IsAnonymous:    false,
		UserID:         input.UserID,
		Username:       input.Username,
		IsVerified:     input.IsVerified,
		ProfilePicture: fmt.Sprintf("https://example.com/%s.jpg", input.Username),
		ChannelSlug:    input.Username,
		Identity:       identity,
	}
}
