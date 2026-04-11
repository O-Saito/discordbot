package bot

import (
	"sort"

	"github.com/bwmarrin/discordgo"
)

type Command interface {
	Name() string
	Description() string
	Execute(g *GuildState, s *discordgo.Session, args *map[string]any) error
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
