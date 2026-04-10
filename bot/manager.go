package bot

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

type Config struct {
	Token           string   `json:"token"`
	VoiceChannelID  string   `json:"voiceChannelId"`
	GuildID         string   `json:"guildId"`
	MusicFolders    []string `json:"musicFolders"`
	RecursiveSearch bool     `json:"recursiveSearch"`
}

type GuildState struct {
	Queue           []string
	IsPlaying       bool
	CurrentTrack    string
	Volume          int
	VoiceChannel    string
	VoiceConnection *discordgo.VoiceConnection
}

type Manager struct {
	session         *discordgo.Session
	token           string
	voiceChannel    string
	guildID         string
	guildState      map[string]*GuildState
	mu              sync.RWMutex
	musicFolders    []string
	recursiveSearch bool
}

func NewManager(token, voiceChannel, guildID string, musicFolders []string, recursiveSearch bool) (*Manager, error) {
	if token == "" {
		return nil, fmt.Errorf("discord token is required")
	}

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	dg.StateEnabled = true
	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildVoiceStates | discordgo.IntentsMessageContent

	return &Manager{
		session:         dg,
		token:           token,
		voiceChannel:    voiceChannel,
		guildID:         guildID,
		guildState:      make(map[string]*GuildState),
		musicFolders:    musicFolders,
		recursiveSearch: recursiveSearch,
	}, nil
}

func (m *Manager) Session() *discordgo.Session {
	return m.session
}

func (m *Manager) MusicFolders() []string {
	return m.musicFolders
}

func (m *Manager) RecursiveSearch() bool {
	return m.recursiveSearch
}

func (m *Manager) Start() error {
	m.session.AddHandler(m.onReady)
	m.session.AddHandler(m.onMessageCreate)
	m.session.AddHandler(m.onGuildLeave)
	m.session.AddHandler(m.onVoiceStateUpdate)

	fmt.Println("Opening Discord session...")
	if err := m.session.Open(); err != nil {
		return fmt.Errorf("failed to open session: %w", err)
	}

	fmt.Println("Bot is now online!")
	return nil
}

func (m *Manager) Stop() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, state := range m.guildState {
		if state.VoiceConnection != nil {
			state.VoiceConnection.Disconnect()
		}
	}

	return m.session.Close()
}

func (m *Manager) GetGuildState(guildID string) *GuildState {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, exists := m.guildState[guildID]; exists {
		return state
	}

	state := &GuildState{
		Queue:     make([]string, 0),
		IsPlaying: false,
		Volume:    100,
	}

	m.guildState[guildID] = state
	fmt.Printf("Created new GuildState for guild: %s\n", guildID)
	return state
}

func (m *Manager) RemoveGuildState(guildID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, exists := m.guildState[guildID]; exists {
		if state.VoiceConnection != nil {
			state.VoiceConnection.Disconnect()
		}
		delete(m.guildState, guildID)
		fmt.Printf("Removed GuildState for guild: %s\n", guildID)
	}
}

func (m *Manager) onReady(s *discordgo.Session, r *discordgo.Ready) {
	fmt.Printf("Logged in as: %s#%s\n", s.State.User.Username, s.State.User.Discriminator)

	if m.voiceChannel != "" && m.guildID != "" {
		state := m.GetGuildState(m.guildID)
		m.handleVoiceJoin(state, m.guildID, m.voiceChannel)
	}

	for _, guild := range s.State.Guilds {
		m.GetGuildState(guild.ID)
	}
}

func (m *Manager) onVoiceStateUpdate(s *discordgo.Session, vs *discordgo.VoiceState) {
	if vs.UserID != s.State.User.ID {
		return
	}

	state := m.GetGuildState(vs.GuildID)

	if vs.ChannelID == "" {
		if state.VoiceConnection != nil {
			state.VoiceConnection.Disconnect()
			state.VoiceConnection = nil
		}
		state.VoiceChannel = ""
		return
	}

	m.handleVoiceJoin(state, vs.GuildID, vs.ChannelID)
}

func (m *Manager) handleVoiceJoin(state *GuildState, guildID, channelID string) {
	if state.VoiceConnection != nil {
		if state.VoiceChannel == channelID {
			return
		}
		state.VoiceConnection.Disconnect()
	}

	vc, err := m.session.ChannelVoiceJoin(guildID, channelID, false, true)
	if err != nil {
		fmt.Printf("Failed to join voice channel: %v\n", err)
		return
	}

	state.VoiceConnection = vc
	state.VoiceChannel = channelID

	fmt.Printf("Joined voice channel: %s in guild: %s\n", channelID, guildID)
}

func (m *Manager) onMessageCreate(s *discordgo.Session, msg *discordgo.MessageCreate) {
	if msg.Author.ID == s.State.User.ID {
		return
	}

	content := msg.Content

	guildID := msg.GuildID
	if guildID == "" {
		return
	}

	state := m.GetGuildState(guildID)

	switch content {
	case "ping":
		s.ChannelMessageSend(msg.ChannelID, "pong")
	case "hello":
		s.ChannelMessageSend(msg.ChannelID, "Hello!")
	case "queue":
		if len(state.Queue) == 0 {
			s.ChannelMessageSend(msg.ChannelID, "Queue is empty")
		} else {
			queueMsg := "Queue:\n"
			for i, track := range state.Queue {
				queueMsg += fmt.Sprintf("%d. %s\n", i+1, track)
			}
			s.ChannelMessageSend(msg.ChannelID, queueMsg)
		}
	case "np", "nowplaying":
		if state.CurrentTrack != "" {
			s.ChannelMessageSend(msg.ChannelID, "Now playing: "+state.CurrentTrack)
		} else {
			s.ChannelMessageSend(msg.ChannelID, "Nothing playing")
		}
	case "volume":
		s.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Volume: %d%%", state.Volume))
	}
}

func (m *Manager) onGuildLeave(s *discordgo.Session, event *discordgo.GuildDelete) {
	m.RemoveGuildState(event.ID)
}

func (m *Manager) WaitForSignal() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	fmt.Println("\nShutting down...")
}
