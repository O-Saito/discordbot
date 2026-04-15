package commands

import (
	"mydiscordbot/bot"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

type StalkCommand struct {
	bot.CommandBase
}

func (c *StalkCommand) Name() string        { return "stalk" }
func (c *StalkCommand) Description() string { return "Record voice (not available)" }

func (c *StalkCommand) GetApplicationCommand() discord.ApplicationCommandCreate {
	return discord.SlashCommandCreate{
		Name:        c.Name(),
		Description: "Voice recording temporarily unavailable",
	}
}

func (c *StalkCommand) ParseInteraction(e *events.ApplicationCommandInteractionCreate) *map[string]any {
	return &map[string]any{}
}

func (c *StalkCommand) Execute(cs *bot.CommandState) error {
	cs.SingleResponse("Voice recording unavailable after migration to DisGo")
	return nil
}

func (c *StalkCommand) HandleButton(cs *bot.CommandState, customID string) error { return nil }
func (c *StalkCommand) HandleSelectMenu(cs *bot.CommandState, customID string, values []string) error {
	return nil
}
func (c *StalkCommand) HandleModalSubmit(cs *bot.CommandState, customID string, data map[string]string) error {
	return nil
}
