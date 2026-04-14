package bot

import (
	"sort"

	"github.com/bwmarrin/discordgo"
)

type CommandState struct {
	G    *GuildState
	S    *discordgo.Session
	Args *map[string]any
	VS   *discordgo.VoiceState
	I    *discordgo.InteractionCreate

	hasResponded bool
}

func (cs *CommandState) SingleRespond(content string) {
	if cs.hasResponded {
		cs.S.InteractionResponseEdit(cs.I.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		return
	}
	cs.S.InteractionRespond(cs.I.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
	cs.hasResponded = true
}

type Command interface {
	Name() string
	Description() string
	Execute(cs *CommandState) error
	ParseInteraction(i *discordgo.InteractionCreate) *map[string]any
	GetApplicationCommand() *discordgo.ApplicationCommand
}

var commandRegistry = make(map[string]Command)

func RegisterCommand(cmd Command) {
	commandRegistry[cmd.Name()] = cmd
}

func GetCommand(name string) Command {
	return commandRegistry[name]

}
func ListCommands() []Command {
	cmds := make([]Command, 0, len(commandRegistry))
	for _, cmd := range commandRegistry {
		cmds = append(cmds, cmd)
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Name() < cmds[j].Name()
	})
	return cmds
}
