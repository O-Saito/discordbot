package bot

import (
	"mydiscordbot/audio"
	"mydiscordbot/domain"
	"sync"

	"github.com/bwmarrin/discordgo"
)

type GuildState struct {
	GuildId         string
	Queue           domain.Queue
	IsPlaying       bool
	CurrentTrack    string
	Volume          int
	VoiceChannel    string
	VoiceConnection *discordgo.VoiceConnection
	Player          *audio.DiscordPlayer
	Manager         *Manager
	FileService     domain.FileService
	YouTubeService  domain.YouTubeService
	ActiveCommands  []*discordgo.ApplicationCommand
	mu              sync.Mutex
}
