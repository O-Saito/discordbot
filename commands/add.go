package commands

import (
	"fmt"
	"mydiscordbot/bot"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

type AddCommand struct {
	bot.CommandBase
}

func (c *AddCommand) Name() string        { return "add" }
func (c *AddCommand) Description() string { return "Add a track to queue (url or search query)" }

func (c *AddCommand) ParseInteraction(e *events.ApplicationCommandInteractionCreate) *map[string]any {
	return &map[string]any{
		"query": e.SlashCommandInteractionData().String("query"),
	}
}

func (c *AddCommand) GetApplicationCommand() discord.ApplicationCommandCreate {
	return discord.SlashCommandCreate{
		Name:        c.Name(),
		Description: c.Description(),
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionString{
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
		cs.SingleResponse("Query required!")
		return fmt.Errorf("usage: add <url or search query>")
	}

	query, _ := (*args)["query"].(string)
	if query == "" {
		cs.SingleResponse("Query required!")
		return nil
	}

	g := cs.G
	if g.VoiceConn == nil {
		cs.SingleResponse("Use /join first!")
		return nil
	}

	cs.SingleResponse("Processing: " + query)
	return nil
}

func (c *AddCommand) HandleButton(cs *bot.CommandState, customID string) error { return nil }
func (c *AddCommand) HandleSelectMenu(cs *bot.CommandState, customID string, values []string) error {
	return nil
}
func (c *AddCommand) HandleModalSubmit(cs *bot.CommandState, customID string, data map[string]string) error {
	return nil
}
