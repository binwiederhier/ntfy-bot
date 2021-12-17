package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Client struct {
	Messages      chan *Message
	subscriptions map[string]*subscription
	mu sync.Mutex
}

type Message struct {
	Event string
	Topic string
	Message string
	Title string
	Priority int
	Tags []string
}

type subscription struct {
	cancel context.CancelFunc
}

func New() *Client {
	return &Client{
		Messages: make(chan *Message),
		subscriptions: make(map[string]*subscription),
	}
}

func (c *Client) Publish(topicURL string, message string, options ...MessageOption) error {
	req, _ := http.NewRequest("POST", topicURL, strings.NewReader(message))
	for _, option := range options {
		if err := option(req); err != nil {
			return err
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode > 299 {
		return fmt.Errorf("unexpected response %d from server", resp.StatusCode)
	}
	return err
}

func (c *Client) Subscribe(topicURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.subscriptions[topicURL]; ok {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.subscriptions[topicURL] = &subscription{cancel}
	go handleConnectionLoop(ctx, c.Messages, topicURL)
}

func (c *Client) Unsubscribe(topicURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sub, ok := c.subscriptions[topicURL]
	if !ok {
		return
	}
	sub.cancel()
	return
}

func handleConnectionLoop(ctx context.Context, msgChan chan *Message, topicURL string) {
	for {
		if err := handleConnection(ctx, msgChan, topicURL); err != nil {
			log.Printf("connection to %s failed: %s", topicURL, err.Error())
		}
		select {
		case <-ctx.Done():
			log.Printf("connection to %s exited", topicURL)
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func handleConnection(ctx context.Context, msgChan chan *Message, topicURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/json", topicURL), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var m *Message
		if err := json.NewDecoder(strings.NewReader(scanner.Text())).Decode(&m); err != nil {
			return err
		}
		msgChan <- m
	}
	return nil
}
