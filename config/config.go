// Package config provides the main configuration for ntfybot
package config

import "strings"

// Platform defines the target chat application platform
type Platform string

// All possible Platform constants
const (
	Slack   = Platform("slack")
	Discord = Platform("discord")
	Mem     = Platform("mem")
)

const (
	DefaultBaseURL = "https://ntfy.sh"
)

type Config struct {
	Token              string
	BaseURL string
	Debug              bool
}

// New instantiates a default new config
func New(token string) *Config {
	return &Config{
		Token:              token,
		BaseURL: DefaultBaseURL,
	}
}

// Platform returns the target connection type, based on the token
func (c *Config) Platform() Platform {
	if strings.HasPrefix(c.Token, "mem") {
		return Mem
	} else if strings.HasPrefix(c.Token, "xoxb-") {
		return Slack
	}
	return Discord
}
