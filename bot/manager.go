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
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/voice"
	"github.com/thomas-vilte/dave-go/session"

	"mydiscordbot/audio"
	"mydiscordbot/config"
	"mydiscordbot/domain"
)

var globalManager *Manager

type Manager struct {
	config     *config.Config
	client     *bot.Client
	guildState map[string]*GuildState
	mu         sync.RWMutex
	ffmpegPath string
}

func NewManager(cfg *config.Config) (*Manager, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("discord token is required")
	}

	client, err := disgo.New(cfg.Token,
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentGuilds,
				gateway.IntentGuildVoiceStates,
				gateway.IntentGuildMessages,
			),
		),
		bot.WithCacheConfigOpts(
			cache.WithCaches(cache.FlagVoiceStates),
		),
		bot.WithVoiceManagerConfigOpts(
			voice.WithDaveSessionCreateFunc(session.New),
		),
		bot.WithEventListenerFunc(func(e *events.Ready) {
			fmt.Printf("Logged in as: %s\n", e.User.Username)
			//globalManager.mu.Lock()
			//globalManager.mu.Unlock()
			for _, guild := range e.Guilds {
				globalManager.GetGuildState(guild.ID.String())
			}
		}),
		bot.WithEventListenerFunc(func(e *events.GuildJoin) {
			fmt.Printf("Guild created/joined: %s (%s)\n", e.Guild.Name, e.GuildID)
			globalManager.GetGuildState(e.GuildID.String())
		}),
		bot.WithEventListenerFunc(func(e *events.ApplicationCommandInteractionCreate) {
			globalManager.handleApplicationCommand(e)
		}),
		bot.WithEventListenerFunc(func(e *events.GuildVoiceStateUpdate) {
			globalManager.handleVoiceStateUpdate(e)
		}),
		bot.WithEventListenerFunc(func(e *events.GuildLeave) {
			fmt.Printf("Guild left: %s\n", e.GuildID)
			globalManager.RemoveGuildState(e.GuildID.String())
		}),
		bot.WithEventListenerFunc(func(e *events.ComponentInteractionCreate) {
			globalManager.handleComponentInteraction(e)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create DisGo client: %w", err)
	}

	m := &Manager{
		client:     client,
		config:     cfg,
		guildState: make(map[string]*GuildState),
		ffmpegPath: "ffmpeg",
	}
	globalManager = m

	return m, nil
}

func (m *Manager) Client() *bot.Client {
	return m.client
}

func (m *Manager) MusicFolders() []string {
	return m.config.MusicFolders
}

func (m *Manager) RecursiveSearch() bool {
	return m.config.RecursiveSearch
}

func (m *Manager) Start() error {
	fmt.Println("Opening Discord gateway...")
	if err := m.client.OpenGateway(context.TODO()); err != nil {
		return fmt.Errorf("failed to open gateway: %w", err)
	}

	fmt.Println("Bot is now online!")
	return nil
}

func (m *Manager) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, state := range m.guildState {
		if state.Player != nil {
			state.Player.Stop()
		}
		if state.VoiceConn != nil {
			state.VoiceConn.Close(ctx)
		}
	}

	m.client.Close(ctx)
	return nil
}

func (m *Manager) GetGuildState(guildID string) *GuildState {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, exists := m.guildState[guildID]; exists {
		fmt.Printf("Returning state: \n")
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
		ActiveCommands: make([]ApplicationCommand, len(commands)),
		Data:           make(map[string]any),
	}

	for i, v := range commands {
		ac := v.GetApplicationCommand()
		if ac == nil {
			fmt.Printf("Command %s does not provide an application command definition\n", v.Name())
			continue
		}

		cmd, err := m.client.Rest.CreateGlobalCommand(m.client.ApplicationID, ac)
		if err != nil {
			log.Panicf("Cannot create '%s' command: %v", v.Name(), err)
		}

		state.ActiveCommands[i] = ApplicationCommand{
			ID:   cmd.ID(),
			Name: cmd.Name(),
		}
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
		if state.VoiceConn != nil {
			state.VoiceConn.Close(context.TODO())
		}
		delete(m.guildState, guildID)
		fmt.Printf("Removed GuildState for guild: %s\n", guildID)
	}
}

func (m *Manager) handleApplicationCommand(e *events.ApplicationCommandInteractionCreate) {
	guildID := e.GuildID().String()
	state := m.GetGuildState(guildID)

	data := e.SlashCommandInteractionData()
	cmdName := data.CommandName()

	cmd := GetCommand(cmdName)
	if cmd == nil {
		e.CreateMessage(discord.NewMessageCreate().WithContent("Comando não encontrado"))
		return
	}

	CommandState := &CommandState{
		G:     state,
		Event: e,
		Args:  cmd.ParseInteraction(e),
	}

	e.CreateMessage(discord.NewMessageCreate().WithContent(fmt.Sprintf("Executando o comando %s", cmdName)))
	fmt.Printf("Executing %s command\n", cmdName)

	go cmd.Execute(CommandState)
}

func (m *Manager) handleVoiceStateUpdate(e *events.GuildVoiceStateUpdate) {
	guildID := e.VoiceState.GuildID.String()
	userID := e.VoiceState.UserID

	//m.mu.Lock()
	//defer m.mu.Unlock()

	state := m.GetGuildState(guildID)

	if userID == m.client.ID() {
		if e.VoiceState.ChannelID == nil || *e.VoiceState.ChannelID == 0 {
			fmt.Printf("Left voice channel in guild: %s\n", guildID)
			if state.VoiceConn != nil {
				//state.VoiceConn.Close(context.TODO())
				//state.VoiceConn = nil
			}
			//state.VoiceChannel = 0
		}
	}
}

func (m *Manager) AddToQueue(guildID string, track domain.Track) {
	state := m.GetGuildState(guildID)
	state.Queue.Enqueue(track)
}

func (m *Manager) WaitForSignal() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	fmt.Println("\nShutting down...")
}

func (m *Manager) handleComponentInteraction(e *events.ComponentInteractionCreate) {
	data := e.Data
	customID := data.CustomID()

	if customID == "" {
		return
	}

	guildID := e.GuildID().String()
	state := m.GetGuildState(guildID)
	state.mu.Lock()

	parts := strings.SplitN(customID, "_", 2)
	if len(parts) < 2 {
		return
	}

	cmdName := parts[0]
	cmd := GetCommand(cmdName)
	if cmd == nil {
		e.CreateMessage(discord.NewMessageCreate().WithContent("Command not found").WithFlags(discord.MessageFlagEphemeral))
		return
	}

	cs := &CommandState{
		G:     state,
		Event: nil,
		Args:  nil,
	}

	state.mu.Unlock()
	switch data := data.(type) {
	case discord.ButtonInteractionData:
		cmd.HandleButton(cs, strings.Join(parts[1:], "_"))
	case discord.StringSelectMenuInteractionData:
		cmd.HandleSelectMenu(cs, strings.Join(parts[1:], "_"), data.Values)
	}
}
