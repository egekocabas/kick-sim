package events

import "time"

type Badge struct {
	Text  string `json:"text"`
	Type  string `json:"type"`
	Count int    `json:"count,omitempty"`
}

type Identity struct {
	UsernameColor string  `json:"username_color"`
	Badges        []Badge `json:"badges"`
}

type User struct {
	IsAnonymous    bool      `json:"is_anonymous"`
	UserID         int64     `json:"user_id"`
	Username       string    `json:"username"`
	IsVerified     bool      `json:"is_verified"`
	ProfilePicture string    `json:"profile_picture"`
	ChannelSlug    string    `json:"channel_slug"`
	Identity       *Identity `json:"identity"`
}

type ChatMessageSent struct {
	MessageID   string `json:"message_id"`
	RepliesTo   any    `json:"replies_to"`
	Broadcaster User   `json:"broadcaster"`
	Sender      User   `json:"sender"`
	Content     string `json:"content"`
	Emotes      []any  `json:"emotes"`
	CreatedAt   string `json:"created_at"`
}

type ChatMessageInput struct {
	MessageID   string
	Content     string
	CreatedAt   time.Time
	Sender      User
	Broadcaster User
}

func NewChatMessageSent(input ChatMessageInput) ChatMessageSent {
	return ChatMessageSent{
		MessageID:   input.MessageID,
		RepliesTo:   nil,
		Broadcaster: input.Broadcaster,
		Sender:      input.Sender,
		Content:     input.Content,
		Emotes:      []any{},
		CreatedAt:   input.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}
