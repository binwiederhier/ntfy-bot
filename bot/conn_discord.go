package bot

import (
	"context"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"ntfy-bot/config"
	"regexp"
	"sync"
)

var (
	discordUserLinkRegex    = regexp.MustCompile(`<@!([^>]+)>`)
	discordChannelLinkRegex = regexp.MustCompile(`<#[^>]+>`)
	discordCodeBlockRegex   = regexp.MustCompile("```([^`]+)```")
	discordCodeRegex        = regexp.MustCompile("`([^`]+)`")
)

type discordConn struct {
	config   *config.Config
	session  *discordgo.Session
	channels map[string]*discordgo.Channel
	mu       sync.Mutex
}

func newDiscordConn(conf *config.Config) *discordConn {
	return &discordConn{
		config:   conf,
		channels: make(map[string]*discordgo.Channel),
	}
}

func (c *discordConn) Connect(ctx context.Context) (<-chan event, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	discord, err := discordgo.New(fmt.Sprintf("Bot %s", c.config.Token))
	if err != nil {
		return nil, err
	}
	eventChan := make(chan event)
	discord.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if ev := c.translateMessageEvent(m); ev != nil {
			eventChan <- ev
		}
	})
	discord.Identify.Intents = discordgo.IntentsGuildMessages
	if err := discord.Open(); err != nil {
		return nil, err
	}
	c.session = discord
	if discord.State == nil || discord.State.User == nil {
		return nil, errors.New("unexpected internal state")
	}
	log.Printf("Discord connected as user %s/%s", discord.State.User.Username, discord.State.User.ID)
	return eventChan, nil
}

func (c *discordConn) Send(channel string, message string) error {
	_, err := c.SendWithID(channel, message)
	return err
}

func (c *discordConn) SendWithID(channel string, message string) (string, error) {
	msg, err := c.session.ChannelMessageSend(channel, message)
	if err != nil {
		return "", err
	}
	return msg.ID, nil
}

func (c *discordConn) React(channelID string, messageID, emoji string) error {
	return c.session.MessageReactionAdd(channelID, messageID, emoji)
}

func (c *discordConn) Close() error {
	return c.session.Close()
}

func (c *discordConn) MentionBot() string {
	return fmt.Sprintf("<@!%s>", c.session.State.User.ID)
}

func (c *discordConn) Mention(user string) string {
	return fmt.Sprintf("<@!%s>", user)
}

func (c *discordConn) ParseMention(user string) (string, error) {
	if matches := discordUserLinkRegex.FindStringSubmatch(user); len(matches) > 0 {
		return matches[1], nil
	}
	return "", errors.New("invalid user")
}

func (c *discordConn) Unescape(s string) string {
	s = discordCodeBlockRegex.ReplaceAllString(s, "$1")
	s = discordCodeRegex.ReplaceAllString(s, "$1")
	s = discordUserLinkRegex.ReplaceAllString(s, "")    // Remove entirely!
	s = discordChannelLinkRegex.ReplaceAllString(s, "") // Remove entirely!
	return s
}

func (c *discordConn) translateMessageEvent(m *discordgo.MessageCreate) event {
	if m.Author.ID == c.session.State.User.ID {
		return nil
	}
	return &messageEvent{
		ID:          m.ID,
		Channel:     m.ChannelID,
		User:        m.Author.ID,
		Message:     m.Content,
	}
}

func (c *discordConn) channel(channel string) (*discordgo.Channel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ch, ok := c.channels[channel]; ok {
		return ch, nil
	}
	ch, err := c.session.Channel(channel)
	if err != nil {
		return nil, err
	}
	c.channels[channel] = ch
	return ch, nil
}

