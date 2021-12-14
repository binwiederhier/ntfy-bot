// Package bot provides the ntfybot main functionality
package bot

import (
	"context"
	_ "embed" // go:embed requires this
	"fmt"
	"golang.org/x/sync/errgroup"
	"log"
	"ntfy-bot/client"
	"ntfy-bot/config"
	"ntfy-bot/util"
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
	b.mu.Lock()
	defer b.mu.Unlock()
	log.Printf("%#v", ev)
	topicURL := "https://ntfy.sh/mytopic"
	if ev.Message == "sub" {
		log.Printf("Subscribing to %s in channel %s", topicURL, ev.Channel)
		if _, ok := b.subscriptions[topicURL]; !ok {
			b.client.Subscribe(topicURL)
			b.subscriptions[topicURL] = make([]string, 0)
		}
		b.subscriptions[topicURL] = append(b.subscriptions[topicURL], ev.Channel)
	} else if ev.Message == "unsub" {
		log.Printf("Unsubscribing from %s in channel %s", topicURL, ev.Channel)
		if _, ok := b.subscriptions[topicURL]; ok {
			b.subscriptions[topicURL] = util.RemoveString(b.subscriptions[topicURL], ev.Channel)
			if len(b.subscriptions[topicURL]) == 0 {
				log.Printf("No more subscriptions to topic %s; terminating connection", topicURL)
				b.client.Unsubscribe(topicURL)
				delete(b.subscriptions, topicURL)
			}
		}
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
	topicURL := "https://ntfy.sh/mytopic"
	if _, ok := b.subscriptions[topicURL]; ok {
		for _, channel := range b.subscriptions[topicURL] {
			b.conn.Send(channel, m.Message)
		}
	}
	return nil
}
