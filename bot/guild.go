package bot

import (
	"mydiscordbot/audio"
	"mydiscordbot/domain"
	"os"
	"sync"

	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
	"layeh.com/gopus"
)

type UserRecording struct {
	File         *os.File
	Decoder      *gopus.Decoder
	DataSize     int64
	WavDataStart int64
}

type GuildState struct {
	GuildId            string
	Queue              domain.Queue
	IsPlaying          bool
	CurrentTrack       string
	Volume             int
	VoiceChannel       snowflake.ID
	VoiceConn          voice.Conn
	Player             *audio.DiscordPlayer
	Manager            *Manager
	ActiveCommands     []ApplicationCommand
	Data               map[string]any
	PlaybackControl    chan string
	PlaybackDone       chan struct{}
	mu                 sync.Mutex
	IsRecording        bool
	RecordingUserFiles map[snowflake.ID]*UserRecording
	RecordingBaseName  string
	RecordingDone      chan struct{}
}

type ApplicationCommand struct {
	ID      snowflake.ID
	Name    string
	GuildID snowflake.ID
}
