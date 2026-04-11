package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/bwmarrin/discordgo"

	"mydiscordbot/audio"
	"mydiscordbot/config"
	"mydiscordbot/domain"
	"mydiscordbot/services/file"
	"mydiscordbot/services/ytdlp"
)

type Manager struct {
	config     *config.Config
	session    *discordgo.Session
	guildState map[string]*GuildState
	mu         sync.RWMutex
	ffmpegPath string
}

func NewManager(config *config.Config) (*Manager, error) {
	if config.Token == "" {
		return nil, fmt.Errorf("discord token is required")
	}

	dg, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	dg.StateEnabled = true
	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildVoiceStates | discordgo.IntentsMessageContent | discordgo.IntentsAll

	return &Manager{
		session:    dg,
		config:     config,
		guildState: make(map[string]*GuildState),
		ffmpegPath: "ffmpeg",
	}, nil
}

func (m *Manager) Session() *discordgo.Session {
	return m.session
}

func (m *Manager) MusicFolders() []string {
	return m.config.MusicFolders
}

func (m *Manager) RecursiveSearch() bool {
	return m.config.RecursiveSearch
}

func (m *Manager) Start() error {
	m.session.AddHandler(m.onReady)
	m.session.AddHandler(m.onMessageCreate)
	m.session.AddHandler(m.onGuildLeave)
	m.session.AddHandler(m.onVoiceStateUpdate)
	m.session.AddHandler(m.onInteractionCreate)

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
		if state.Player != nil {
			state.Player.Stop()
		}
		if state.VoiceConnection != nil {
			state.VoiceConnection.Disconnect()
		}

		for _, v := range state.ActiveCommands {
			err := m.session.ApplicationCommandDelete(m.config.ApplicationID, state.GuildId, v.ID)
			if err != nil {
				log.Panicf("Cannot delete '%v' command: %v", v.Name, err)
			}
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

	player, err := audio.NewDiscordPlayer(m.ffmpegPath)
	if err != nil {
		fmt.Printf("Failed to create audio player: %v\n", err)
		player = nil
	}

	commands := ListCommands()

	state := &GuildState{
		GuildId:        guildID,
		Queue:          domain.NewQueue(100),
		IsPlaying:      false,
		Volume:         100,
		Player:         player,
		Manager:        m,
		FileService:    file.New(),
		YouTubeService: ytdlp.New(),
		ActiveCommands: make([]*discordgo.ApplicationCommand, len(commands)),
	}

	for i, v := range commands {
		ac := v.GetApplicationCommand()
		if ac == nil {
			fmt.Printf("Command %s does not provide an application command definition\n", v.Name())
			continue
		}
		m.session.ApplicationCommandCreate(m.config.ApplicationID, state.GuildId, ac)
		if err != nil {
			log.Panicf("Cannot create '%s' command: %v", v.Name(), err)
		}
		state.ActiveCommands[i] = ac
	}

	m.guildState[guildID] = state
	fmt.Printf("Created new GuildState for guild: %s\n", guildID)
	return state
}

func (m *Manager) RemoveGuildState(guildID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, exists := m.guildState[guildID]; exists {
		if state.Player != nil {
			state.Player.Stop()
		}
		if state.VoiceConnection != nil {
			state.VoiceConnection.Disconnect()
		}
		delete(m.guildState, guildID)
		fmt.Printf("Removed GuildState for guild: %s\n", guildID)
	}
}

func (m *Manager) onReady(s *discordgo.Session, r *discordgo.Ready) {
	fmt.Printf("Logged in as: %s#%s\n", s.State.User.Username, s.State.User.Discriminator)
	//m.mu.Lock()
	//defer m.mu.Unlock()

	if m.config.VoiceChannelID != "" && m.config.GuildID != "" {
		state := m.GetGuildState(m.config.GuildID)
		m.handleVoiceJoin(state, m.config.VoiceChannelID)

		cmd := GetCommand("add")
		if cmd == nil {
			fmt.Printf("Add command not found\n")
			return
		}
		//cmd.Execute(state, s, &map[string]any{"query": "something"})
	}
}

func (m *Manager) onVoiceStateUpdate(s *discordgo.Session, vs *discordgo.VoiceState) {
	fmt.Printf("onVoiceStateUpdate: UserID=%s, GuildID=%s, ChannelID=%s\n", vs.UserID, vs.GuildID, vs.ChannelID)
	m.mu.Lock()
	defer m.mu.Unlock()

	if vs.UserID != s.State.User.ID {
		return
	}

	state := m.GetGuildState(vs.GuildID)

	if vs.ChannelID == "" {
		fmt.Printf("Left voice channel in guild: %s\n", vs.GuildID)
		if state.VoiceConnection != nil {
			state.VoiceConnection.Disconnect()
			state.VoiceConnection = nil
		}
		state.VoiceChannel = ""
		return
	}

}

func (m *Manager) onMessageCreate(s *discordgo.Session, msg *discordgo.MessageCreate) {
	fmt.Printf("onMessageCreate: UserID=%s, GuildID=%s, Content=%s\n", msg.Author.ID, msg.GuildID, msg.Content)
	m.mu.Lock()
	defer m.mu.Unlock()

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
	case "play":
		if err := m.Play(guildID); err != nil {
			s.ChannelMessageSend(msg.ChannelID, "Error: "+err.Error())
		} else {
			s.ChannelMessageSend(msg.ChannelID, "Playing: "+state.CurrentTrack)
		}
	case "stop":
		m.StopPlay(guildID)
		s.ChannelMessageSend(msg.ChannelID, "Stopped")
	case "pause":
		m.Pause(guildID)
		s.ChannelMessageSend(msg.ChannelID, "Paused")
	case "resume":
		m.Resume(guildID)
		s.ChannelMessageSend(msg.ChannelID, "Resumed")
	case "skip":
		m.playNext(guildID)
		s.ChannelMessageSend(msg.ChannelID, "Skipped")
	case "queue":
		if state.Queue.IsEmpty() {
			s.ChannelMessageSend(msg.ChannelID, "Queue is empty")
		} else {
			tracks := state.Queue.All()
			queueMsg := "Queue:\n"
			for i, track := range tracks {
				queueMsg += fmt.Sprintf("%d. %s\n", i+1, track.Title())
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
	default:
		if strings.HasPrefix(content, "play ") {
			query := strings.TrimPrefix(content, "play ")
			m.handlePlayCommand(s, msg.ChannelID, guildID, query)
		}
	}
}

func (m *Manager) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	state := m.GetGuildState(i.GuildID)
	state.mu.Lock()
	defer state.mu.Unlock()

	cmd := GetCommand(i.ApplicationCommandData().Name)
	if cmd == nil {
		fmt.Printf("%s command not found\n", i.ApplicationCommandData().Name)
		return

	}

	fmt.Printf("Executing %s command\n", i.ApplicationCommandData().Name)
	cmd.Execute(state, s, cmd.ParseInteraction(i))
}

func (m *Manager) handleVoiceJoin(state *GuildState, channelID string) {
	fmt.Printf("Handling voice join for guild: %s, channel: %s\n", state.GuildId, channelID)

	if state.VoiceConnection != nil {
		if state.VoiceChannel == channelID {
			return
		}
		state.VoiceConnection.Disconnect()
	}

	vc, err := m.session.ChannelVoiceJoin(state.GuildId, channelID, false, true)
	if err != nil {
		fmt.Printf("Failed to join voice channel: %v\n", err)
		return
	}

	state.VoiceConnection = vc
	state.VoiceChannel = channelID

	fmt.Printf("Joined voice channel: %s in guild: %s\n", channelID, state.GuildId)
}

func (m *Manager) handlePlayCommand(s *discordgo.Session, channelID, guildID, query string) {
	fmt.Printf("Handling play command for guild: %s, query: %s\n", guildID, query)
	state := m.GetGuildState(guildID)

	if len(m.config.MusicFolders) > 0 {
		fileSvc := file.New()
		results, err := fileSvc.Search(m.config.MusicFolders, query, m.config.RecursiveSearch)
		if err == nil && len(results) > 0 {
			state.Queue.Enqueue(results[0])
			s.ChannelMessageSend(channelID, "Added to queue: "+results[0].Title())
			if !state.IsPlaying {
				m.Play(guildID)
			}
			return
		}
	}

	ytSvc := ytdlp.New()
	results, err := ytSvc.Search(context.Background(), query, 5)
	if err != nil || len(results) == 0 {
		s.ChannelMessageSend(channelID, "No results found")
		return
	}

	track, err := ytSvc.ParseURL(context.Background(), results[0].URL)
	if err != nil {
		s.ChannelMessageSend(channelID, "Error parsing URL: "+err.Error())
		return
	}

	audioURL, err := ytSvc.GetAudioURL(context.Background(), results[0].URL)
	if err != nil {
		s.ChannelMessageSend(channelID, "Error getting audio URL: "+err.Error())
		return
	}

	track.SetAudioURL(audioURL)
	state.Queue.Enqueue(track)
	s.ChannelMessageSend(channelID, "Added to queue: "+track.Title())

	if !state.IsPlaying {
		m.Play(guildID)
	}
}

func (m *Manager) AddToQueue(guildID string, track domain.Track) {
	state := m.GetGuildState(guildID)
	state.Queue.Enqueue(track)
}

func (m *Manager) Play(guildID string) error {
	state := m.GetGuildState(guildID)

	if state.Queue.IsEmpty() {
		return fmt.Errorf("queue is empty")
	}

	if state.VoiceConnection == nil {
		return fmt.Errorf("not in a voice channel")
	}

	track, err := state.Queue.Dequeue()
	if err != nil {
		return err
	}

	state.CurrentTrack = track.Title()
	state.IsPlaying = true

	state.Player.SetOnFinishedCallback(func() {
		m.playNext(guildID)
	})

	return state.Player.PlayURLWithSeekAndVC(track.AudioURL(), 48000, 0, state.VoiceConnection)
}

func (m *Manager) playNext(guildID string) {
	state := m.GetGuildState(guildID)

	if state.Queue.IsEmpty() {
		state.CurrentTrack = ""
		state.IsPlaying = false
		return
	}

	state.Player.SetOnFinishedCallback(func() {
		m.playNext(guildID)
	})

	track, err := state.Queue.Dequeue()
	if err != nil {
		state.CurrentTrack = ""
		state.IsPlaying = false
		return
	}

	state.CurrentTrack = track.Title()
	state.Player.PlayURLWithSeekAndVC(track.AudioURL(), 48000, 0, state.VoiceConnection)
}

func (m *Manager) StopPlay(guildID string) {
	state := m.GetGuildState(guildID)
	if state.Player != nil {
		state.Player.Stop()
	}
	state.IsPlaying = false
	state.CurrentTrack = ""
}

func (m *Manager) Pause(guildID string) {
	state := m.GetGuildState(guildID)
	if state.Player != nil {
		state.Player.Pause()
	}
	state.IsPlaying = false
}

func (m *Manager) Resume(guildID string) {
	state := m.GetGuildState(guildID)
	if state.Player != nil {
		state.Player.Resume()
	}
	state.IsPlaying = true
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
