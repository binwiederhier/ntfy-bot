// Package bot provides the ntfybot main functionality
package bot

import (
	"bytes"
	"context"
	_ "embed" // go:embed requires this
	"errors"
	"fmt"
	"github.com/google/shlex"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"
	"log"
	"ntfy-bot/client"
	"ntfy-bot/config"
	"ntfy-bot/util"
	"strings"
	"sync"
)

// Bot is the main struct that provides the bot
type Bot struct {
	config    *config.Config
	conn      conn
	client *client.Client
	subscriptions map[string][]string // Topic URL -> Channel IDs
	cancelFn  context.CancelFunc
	mu        sync.RWMutex
}

// New creates a new REPLbot instance using the given configuration
func New(conf *config.Config) (*Bot, error) {
	var conn conn
	switch conf.Platform() {
	case config.Discord:
		conn = newDiscordConn(conf)
	default:
		return nil, fmt.Errorf("invalid type: %s", conf.Platform())
	}
	return &Bot{
		config:    conf,
		conn:      conn,
		client: client.New(),
		subscriptions: make(map[string][]string),
	}, nil
}

// Run runs the bot in the foreground indefinitely or until Stop is called.
// This method does not return unless there is an error, or if gracefully shut down via Stop.
func (b *Bot) Run() error {
	var ctx context.Context
	ctx, b.cancelFn = context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	eventChan, err := b.conn.Connect(ctx)
	if err != nil {
		return err
	}
	g.Go(func() error {
		return b.handleChatEvents(ctx, eventChan)
	})
	g.Go(func() error {
		return b.handleSubscriptionMessages(ctx, b.client.Messages)
	})
	return g.Wait()
}

// Stop gracefully shuts down the bot, closing all active sessions gracefully
func (b *Bot) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cancelFn() // This must be at the end, see app.go
}

func (b *Bot) handleChatEvents(ctx context.Context, eventChan <-chan event) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-eventChan:
			if err := b.handleChatEvent(ev); err != nil {
				return err
			}
		}
	}
}

func (b *Bot) handleChatEvent(e event) error {
	switch ev := e.(type) {
	case *messageEvent:
		return b.handleChatMessageEvent(ev)
	case *errorEvent:
		return ev.Error
	default:
		return nil // Ignore other events
	}
}

func (b *Bot) handleChatMessageEvent(ev *messageEvent) error {
	log.Printf("%#v", ev)
	fields := strings.Fields(ev.Message)
	if len(fields) == 0 || fields[0] != b.conn.MentionBot() {
		return nil
	}
	args, err := shlex.Split(ev.Message)
	if err != nil {
		return err
	}
	if err := b.runCLI(ev.Channel, args); err != nil {
		return b.conn.Send(ev.Channel, err.Error())
	}
	return nil
}

func (b *Bot) handleSubscriptionMessages(ctx context.Context, messageChan <-chan *client.Message) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case m := <-messageChan:
			if err := b.handleSubscriptionMessage(m); err != nil {
				return err
			}
		}
	}
}

func (b *Bot) handleSubscriptionMessage(m *client.Message) error {
	if m.Event != "message" {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	log.Printf("Forwarding incoming message to chat: %s", m.Message)
	topicURL := "https://ntfy.sh/" + m.Topic
	message := fmt.Sprintf("**%s**\n%s", util.ShortURL(topicURL), m.Message)
	if _, ok := b.subscriptions[topicURL]; ok {
		for _, channel := range b.subscriptions[topicURL] {
			b.conn.Send(channel, message)
		}
	}
	return nil
}

func (b *Bot) runCLI(channel string, args []string) error {
	var buf bytes.Buffer
	app := &cli.App{
		Name:                   "ntfy",
		Usage:                  "Bot for sending and receiving messages to/from ntfy",
		UsageText:              "ntfy [OPTION..]",
		UseShortOptionHandling: true,
		Reader: &buf,
		Writer: &buf,
		ErrWriter: &buf,
		Commands: []*cli.Command{
			{
				Name:      "subscribe",
				Aliases:   []string{"sub", "add"},
				Usage:     "xxxxxxx",
				UsageText: "ntfy subscribe [--server=...] TOPIC",
				Action:    func (c *cli.Context) error {
					return b.execSubscribe(c, channel)
				},
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "server", Aliases: []string{"s"}, Value: b.config.BaseURL, Usage: "server URL"},
				},
				Description: `xxxxxxxxx`,
			},
			{
				Name:      "unsubscribe",
				Aliases:   []string{"del", "rm"},
				Usage:     "xxxxxxx",
				UsageText: "ntfy unsubscribe [--server=...] TOPIC",
				Action:    func (c *cli.Context) error {
					return b.execUnsubscribe(c, channel)
				},
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "server", Aliases: []string{"s"}, Value: b.config.BaseURL, Usage: "server URL"},
				},
				Description: `xxxxxxxxx`,
			},
		},
		CommandNotFound: func(c *cli.Context, s string) {
			if err := b.execCommandNotFound(c, channel, s); err != nil {
				log.Printf("error executing command not found function: %s", err.Error())
			}
		},
	}
	err := app.Run(args)
	if buf.Len() > 0 {
		_ = b.conn.Send(channel, buf.String())
	}
	return err
}

func (b *Bot) execSubscribe(c *cli.Context, channel string) error {
	baseURL := c.String("server")
	if c.NArg() < 1 {
		return errors.New("missing server address, see --help for usage details")
	}
	topic := c.Args().First()
	topicURL := fmt.Sprintf("%s/%s", baseURL, topic)
	log.Printf("Subscribing to %s in channel %s", topicURL, channel)
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subscriptions[topicURL]; !ok {
		b.client.Subscribe(topicURL)
		b.subscriptions[topicURL] = make([]string, 0)
	}
	b.subscriptions[topicURL] = append(b.subscriptions[topicURL], channel)
	return b.conn.Send(channel, fmt.Sprintf("Subscribed to %s", util.ShortURL(topicURL)))
}

func (b *Bot) execUnsubscribe(c *cli.Context, channel string) error {
	baseURL := c.String("server")
	if c.NArg() < 1 {
		return errors.New("missing server address, see --help for usage details")
	}
	topic := c.Args().First()
	topicURL := fmt.Sprintf("%s/%s", baseURL, topic)
	log.Printf("Unsubscribing from %s in channel %s", topicURL, channel)
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subscriptions[topicURL]; ok {
		b.subscriptions[topicURL] = util.RemoveString(b.subscriptions[topicURL], channel)
		if len(b.subscriptions[topicURL]) == 0 {
			log.Printf("No more subscriptions to topic %s; terminating connection", topicURL)
			b.client.Unsubscribe(topicURL)
			delete(b.subscriptions, topicURL)
		}
	}
	return b.conn.Send(channel, fmt.Sprintf("Unsubscribed from %s", util.ShortURL(topicURL)))
}

func (b *Bot) execCommandNotFound(c *cli.Context, channel string, s string) error {
	return b.conn.Send(channel, "command not found")
}

