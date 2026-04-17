package bot

import (
	"mydiscordbot/audio"
	"mydiscordbot/domain"
	"os"
	"sync"
	"time"

	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
	"layeh.com/gopus"
)

const (
	sampleRate = 48000
	channels   = 2
	frameSize  = 960
	maxBytes   = frameSize * 2 * 2
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
	AudioInput         chan []int16
	audioSenderDone    chan struct{}
	audioSenderStopped chan struct{}
	opusEncoder        *gopus.Encoder
	playerMu           sync.Mutex
}

type ApplicationCommand struct {
	ID      snowflake.ID
	Name    string
	GuildID snowflake.ID
}

func (g *GuildState) OpenAudioStream() chan []int16 {
	g.playerMu.Lock()
	defer g.playerMu.Unlock()

	if g.AudioInput != nil {
		return g.AudioInput
	}

	encoder, err := gopus.NewEncoder(sampleRate, channels, gopus.Audio)
	if err != nil {
		return nil
	}
	g.opusEncoder = encoder

	g.AudioInput = make(chan []int16, 5)
	g.audioSenderDone = make(chan struct{})
	g.audioSenderStopped = make(chan struct{})

	go g.audioSender()

	return g.AudioInput
}

func (g *GuildState) audioSender() {
	defer close(g.audioSenderStopped)

	if g.VoiceConn == nil {
		select {
		case <-g.audioSenderDone:
		}
		return
	}

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	silence := make([]int16, frameSize*channels)

	for {
		select {
		case <-g.audioSenderDone:
			return
		case <-ticker.C:
			var pcm []int16
			select {
			case pcm = <-g.AudioInput:
			default:
				pcm = silence
			}

			if pcm == nil {
				continue
			}

			opus, err := g.opusEncoder.Encode(pcm, frameSize, maxBytes)
			if err != nil {
				continue
			}

			if g.VoiceConn == nil || g.VoiceConn.UDP() == nil {
				continue
			}

			_, err = g.VoiceConn.UDP().Write(opus)
			if err != nil {
				return
			}
		}
	}
}

func (g *GuildState) CloseAudioStream() {
	g.playerMu.Lock()
	defer g.playerMu.Unlock()

	if g.audioSenderDone != nil {
		close(g.audioSenderDone)
		g.audioSenderDone = nil
	}

	if g.audioSenderStopped != nil {
		<-g.audioSenderStopped
		g.audioSenderStopped = nil
	}

	if g.AudioInput != nil {
		close(g.AudioInput)
		g.AudioInput = nil
	}
}
