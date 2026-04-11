package commands

import (
	"fmt"
	"mydiscordbot/bot"

	"github.com/bwmarrin/discordgo"
)

const (
	dmPermission                   = false
	defaultMemberPermissions int64 = discordgo.PermissionManageGuild
)

var integerOptionMinValue float64 = 1.0

type ExampleCommand struct{}

func (c *ExampleCommand) Name() string        { return "example" }
func (c *ExampleCommand) Description() string { return "Example command" }
func (c *ExampleCommand) ParseInteraction(i *discordgo.InteractionCreate) *map[string]any {
	return nil
}

func (c *ExampleCommand) GetApplicationCommand() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        c.Name(),
		Description: c.Description(),
		Options: []*discordgo.ApplicationCommandOption{

			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "string-option",
				Description: "String option",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "integer-option",
				Description: "Integer option",
				MinValue:    &integerOptionMinValue,
				MaxValue:    10,
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionNumber,
				Name:        "number-option",
				Description: "Float option",
				MaxValue:    10.1,
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "bool-option",
				Description: "Boolean option",
				Required:    true,
			},

			// Required options must be listed first since optional parameters
			// always come after when they're used.
			// The same concept applies to Discord's Slash-commands API

			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel-option",
				Description: "Channel option",
				// Channel type mask
				ChannelTypes: []discordgo.ChannelType{
					discordgo.ChannelTypeGuildText,
					discordgo.ChannelTypeGuildVoice,
				},
				Required: false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "user-option",
				Description: "User option",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionRole,
				Name:        "role-option",
				Description: "Role option",
				Required:    false,
			},
		},
	}
}

func (c *ExampleCommand) Execute(g *bot.GuildState, s *discordgo.Session, args *map[string]any) error {
	fmt.Printf("Executing example command \r\n")
	return nil
}
