package bot

type event interface{}

type messageEvent struct {
	ID          string
	Channel     string
	User        string
	Message     string
	File        []byte // used for tests only
}

type channelJoinedEvent struct {
	Channel string
}

type errorEvent struct {
	Error error
}

