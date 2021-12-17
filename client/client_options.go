package client

import "net/http"

type MessageOption func(r *http.Request) error

func WithTitle(title string) MessageOption {
	return func(r *http.Request) error {
		if title != "" {
			r.Header.Set("X-Title", title)
		}
		return nil
	}
}

func WithPriority(priority string) MessageOption {
	return func(r *http.Request) error {
		if priority != "" {
			r.Header.Set("X-Priority", priority)
		}
		return nil
	}
}

func WithTags(tags string) MessageOption {
	return func(r *http.Request) error {
		if tags != "" {
			r.Header.Set("X-Tags", tags)
		}
		return nil
	}
}
