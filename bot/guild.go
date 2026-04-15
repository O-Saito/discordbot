package bot

import (
	"mydiscordbot/audio"
	"mydiscordbot/domain"
	"sync"

	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
)

type GuildState struct {
	GuildId         string
	Queue           domain.Queue
	IsPlaying       bool
	CurrentTrack    string
	Volume          int
	VoiceChannel    snowflake.ID
	VoiceConn       voice.Conn
	Player          *audio.DiscordPlayer
	Manager         *Manager
	ActiveCommands  []ApplicationCommand
	Data            map[string]any
	PlaybackControl chan string
	PlaybackDone    chan struct{}
	mu              sync.Mutex
}

type ApplicationCommand struct {
	ID      snowflake.ID
	Name    string
	GuildID snowflake.ID
}
