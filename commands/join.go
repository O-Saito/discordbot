package commands

import (
	"context"
	"fmt"
	"mydiscordbot/bot"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

type JoinCommand struct {
	bot.CommandBase
}

func (c *JoinCommand) Name() string        { return "join" }
func (c *JoinCommand) Description() string { return "Join a voice channel" }

func (c *JoinCommand) GetApplicationCommand() discord.ApplicationCommandCreate {
	return discord.SlashCommandCreate{
		Name:        c.Name(),
		Description: c.Description(),
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionChannel{
				Name:         "channel",
				Description:  "Voice channel to join",
				ChannelTypes: []discord.ChannelType{discord.ChannelTypeGuildVoice},
				Required:     false,
			},
			discord.ApplicationCommandOptionUser{
				Name:        "user",
				Description: "Join the voice channel this user is in",
				Required:    false,
			},
		},
	}
}

func (c *JoinCommand) ParseInteraction(e *events.ApplicationCommandInteractionCreate) *map[string]any {
	opts := make(map[string]any)
	data := e.SlashCommandInteractionData()

	opts["channel"] = data.Channel("channel").ID
	opts["user"] = data.User("user").ID

	if member := e.Member(); member != nil {
		opts["member"] = member.User.ID
	}
	return &opts
}

func (c *JoinCommand) Execute(cs *bot.CommandState) error {
	args := cs.Args
	g := cs.G

	guildID := snowflake.MustParse(g.GuildId)
	var channelID snowflake.ID

	// Try channel from args
	if ch, ok := (*args)["channel"].(snowflake.ID); ok && ch != 0 {
		channelID = ch
	}

	// If no channel, try user or member
	if channelID == 0 {
		var userID snowflake.ID

		// Try user from args
		if user, ok := (*args)["user"].(snowflake.ID); ok && user != 0 {
			userID = user
		}

		// Fallback to member (command user)
		if userID == 0 {
			if member, ok := (*args)["member"].(snowflake.ID); ok && member != 0 {
				userID = member
			}
		}

		// Find user's voice channel from cache
		if userID != 0 && cs.Client != nil {
			caches := cs.Client.Caches
			voiceStates := caches.VoiceStates(snowflake.MustParse(g.GuildId))
			for vs := range voiceStates {
				if vs.UserID == userID && vs.ChannelID != nil {
					channelID = *vs.ChannelID
					break
				}
			}
		}
	}

	if channelID == 0 {
		cs.SingleResponse("Please specify a voice channel or join one yourself")
		return nil
	}

	if g.VoiceConn != nil && g.VoiceChannel != 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		g.VoiceConn.Close(ctx)
		cancel()
	}

	if g.VoiceConn == nil {
		conn := cs.G.Manager.Client().VoiceManager.CreateConn(guildID)
		g.VoiceConn = conn
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := g.VoiceConn.Open(ctx, channelID, false, false); err != nil {
		fmt.Printf("Error joining channel: %v \n", err)
		cs.SingleResponse(fmt.Sprintf("Error joining channel: %v", err))
		return nil
	}

	g.VoiceChannel = channelID
	cs.SingleResponse("Joined voice channel")
	return nil
}

func (c *JoinCommand) HandleButton(cs *bot.CommandState, customID string) error { return nil }
func (c *JoinCommand) HandleSelectMenu(cs *bot.CommandState, customID string, values []string) error {
	return nil
}
func (c *JoinCommand) HandleModalSubmit(cs *bot.CommandState, customID string, data map[string]string) error {
	return nil
}
