package commands

import (
	"fmt"
	"mydiscordbot/bot"

	"github.com/bwmarrin/discordgo"
)

type JoinCommand struct{}

func (c *JoinCommand) Name() string        { return "join" }
func (c *JoinCommand) Description() string { return "Join a voice channel" }

func (c *JoinCommand) GetApplicationCommand() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        c.Name(),
		Description: c.Description(),
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "Voice channel to join",
				ChannelTypes: []discordgo.ChannelType{
					discordgo.ChannelTypeGuildVoice,
				},
				Required: false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "user",
				Description: "Join the voice channel this user is in",
				Required:    false,
			},
		},
	}
}

func (c *JoinCommand) ParseInteraction(i *discordgo.InteractionCreate) *map[string]any {
	opts := make(map[string]any)

	if len(i.ApplicationCommandData().Options) > 0 {
		for _, opt := range i.ApplicationCommandData().Options {
			if opt.Name == "channel" {
				opts["channel"] = opt.ChannelValue(nil)
			} else if opt.Name == "user" {
				opts["user"] = opt.UserValue(nil)
			}
		}
	}

	if i.Member != nil {
		opts["member"] = i.Member
	}

	return &opts
}

func (c *JoinCommand) Execute(cs *bot.CommandState) error {
	args := cs.Args
	g := cs.G
	s := cs.S

	channelID := ""

	channelOpt, hasChannel := (*args)["channel"].(*discordgo.Channel)
	userOpt, hasUser := (*args)["user"].(*discordgo.User)
	memberOpt, hasMember := (*args)["member"].(*discordgo.Member)

	if hasChannel && channelOpt != nil {
		channelID = channelOpt.ID
	} else if hasUser && userOpt != nil {
		vState, err := s.State.VoiceState(g.GuildId, userOpt.ID)
		if err != nil || vState == nil || vState.ChannelID == "" {
			cs.SingleRespond("User is not in a voice channel")
			return nil
		}
		channelID = vState.ChannelID
	} else if hasMember && memberOpt != nil {
		vState, err := s.State.VoiceState(g.GuildId, memberOpt.User.ID)
		if err != nil || vState == nil || vState.ChannelID == "" {
			cs.SingleRespond("You are not in a voice channel")
			return nil
		}
		channelID = vState.ChannelID
	} else {
		cs.SingleRespond("You are not in a voice channel")
		return nil
	}

	channel, err := s.Channel(channelID)
	if err != nil {
		cs.SingleRespond(fmt.Sprintf("Error finding channel: %v", err))
		return nil
	}

	vc, err := s.ChannelVoiceJoin(g.GuildId, channelID, false, true)
	if err != nil {
		cs.SingleRespond(fmt.Sprintf("Error joining channel: %v", err))
		return nil
	}

	g.VoiceConnection = vc
	g.VoiceChannel = channelID

	cs.SingleRespond(fmt.Sprintf("Joined voice channel: %s", channel.Name))

	return nil
}
