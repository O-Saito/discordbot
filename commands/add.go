package commands

import (
	"fmt"
	"mydiscordbot/bot"

	"github.com/bwmarrin/discordgo"
)

type AddCommand struct{}

func (c *AddCommand) Name() string        { return "add" }
func (c *AddCommand) Description() string { return "Add a track to queue (url or search query)" }

func (c *AddCommand) ParseInteraction(i *discordgo.InteractionCreate) *map[string]any {
	return &map[string]any{
		"query": i.ApplicationCommandData().Options[0].StringValue(),
	}
}

func (c *AddCommand) GetApplicationCommand() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        c.Name(),
		Description: c.Description(),
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "query",
				Description: "YouTube URL or search query",
				Required:    true,
			},
		},
	}
}

func (c *AddCommand) Execute(cs *bot.CommandState) error {
	args := cs.Args
	if args == nil || len(*args) == 0 {
		cs.SingleRespond("Parametro query não informado!")
		return fmt.Errorf("usage: add <url or search query>")
	}

	return handlePlay(cs, args)
}
