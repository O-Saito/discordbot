package bot

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/bwmarrin/discordgo"

	"mydiscordbot/audio"
	"mydiscordbot/config"
	"mydiscordbot/domain"
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
	m.session.AddHandler(m.onGuildCreate)
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
		ActiveCommands: make([]*discordgo.ApplicationCommand, len(commands)),
		Data:           make(map[string]any),
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

	err := s.UpdateStatusComplex(discordgo.UpdateStatusData{
		Status: string(discordgo.StatusOnline),
		Activities: []*discordgo.Activity{
			{
				Name: "waiting for command!",
				Type: discordgo.ActivityTypeWatching,
			},
		},
	})
	if err != nil {
		fmt.Printf("Failed to set status: %v\n", err)
	} else {
		fmt.Printf("Bot status set to: Idle: waiting for command!\n")
	}

	s.State.RLock()
	for _, guild := range s.State.Guilds {
		m.GetGuildState(guild.ID)
		fmt.Printf("Registered guild: %s (%s)\n", guild.Name, guild.ID)
	}
	s.State.RUnlock()

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

func (m *Manager) onGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	fmt.Printf("Guild created/joined: %s (%s)\n", g.Name, g.ID)
	m.GetGuildState(g.ID)
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

	guildID := msg.GuildID
	if guildID == "" {
		return
	}

}

func (m *Manager) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	state := m.GetGuildState(i.GuildID)
	state.mu.Lock()
	defer state.mu.Unlock()

	cmd := GetCommand(i.ApplicationCommandData().Name)
	if cmd == nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Comando não encontrado",
			},
		})
		fmt.Printf("%s command not found\n", i.ApplicationCommandData().Name)
		return
	}

	CommandState := &CommandState{
		G:    state,
		S:    s,
		I:    i,
		Args: cmd.ParseInteraction(i),
	}

	CommandState.SingleRespond(fmt.Sprintf("Executando o comando %s", i.ApplicationCommandData().Name))
	fmt.Printf("Executing %s command\n", i.ApplicationCommandData().Name)

	cmd.Execute(CommandState)
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

func (m *Manager) AddToQueue(guildID string, track domain.Track) {
	state := m.GetGuildState(guildID)
	state.Queue.Enqueue(track)
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
