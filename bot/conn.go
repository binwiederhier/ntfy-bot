package bot

import (
	"context"
)

type conn interface {
	Connect(ctx context.Context) (<-chan event, error)
	Send(channel string, message string) error
	SendWithID(channel string, message string) (string, error)
	React(channelID string, messageID, emoji string) error
	MentionBot() string
	Mention(user string) string
	ParseMention(user string) (string, error)
	Unescape(s string) string
	Close() error
}
