package bot

import (
	"sort"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

type CommandState struct {
	G             *GuildState
	Event         *events.ApplicationCommandInteractionCreate
	Args          *map[string]any
	lastMessageID snowflake.ID
	hasResponded  bool
}

func (cs *CommandState) MessageChannelID() snowflake.ID {
	ch := cs.Event.Channel()
	return ch.ID()
}

func (cs *CommandState) GuildID() snowflake.ID {
	gid := cs.Event.GuildID()
	if gid != nil {
		return *gid
	}
	return 0
}

func (cs *CommandState) Member() *discord.ResolvedMember {
	return cs.Event.Member()
}

func (cs *CommandState) SingleResponse(content string) {
	channelID := cs.MessageChannelID()

	if cs.hasResponded && cs.lastMessageID != 0 {
		cs.UpdateMessage(cs.lastMessageID, content, nil, nil)
		return
	}

	msg, err := cs.Event.Client().Rest.CreateMessage(channelID, discord.NewMessageCreate().WithContent(content))
	if err != nil {
		return
	}
	cs.hasResponded = true
	cs.lastMessageID = msg.ID
}

func (cs *CommandState) Respond(content string) {
	cs.SingleResponse(content)
}

func (cs *CommandState) RespondEphemeral(content string) {
	if cs.hasResponded && cs.lastMessageID != 0 {
		channelID := cs.MessageChannelID()
		cs.Event.Client().Rest.UpdateMessage(channelID, cs.lastMessageID,
			discord.NewMessageUpdate().WithContent(content).WithFlags(discord.MessageFlagEphemeral))
		return
	}
	cs.Event.CreateMessage(discord.NewMessageCreate().WithContent(content).WithFlags(discord.MessageFlagEphemeral))
	cs.hasResponded = true
}

func (cs *CommandState) RespondWithEmbed(embed discord.Embed) {
	if cs.hasResponded {
		return
	}
	channelID := cs.MessageChannelID()
	cs.Event.Client().Rest.CreateMessage(channelID, discord.NewMessageCreate().WithEmbeds(embed))
	cs.hasResponded = true
}

func (cs *CommandState) RespondWithComponents(content string, components []discord.LayoutComponent) {
	if cs.hasResponded {
		return
	}
	channelID := cs.MessageChannelID()
	msg, err := cs.Event.Client().Rest.CreateMessage(channelID, discord.NewMessageCreate().
		WithContent(content).
		WithComponents(components...))
	if err != nil {
		return
	}
	cs.hasResponded = true
	cs.lastMessageID = msg.ID
}

func (cs *CommandState) SendEmbed(embed discord.Embed) {
	channelID := cs.MessageChannelID()
	cs.Event.Client().Rest.CreateMessage(channelID, discord.NewMessageCreate().WithEmbeds(embed))
}

func (cs *CommandState) SendMessage(content string, embeds []discord.Embed, components []discord.LayoutComponent) {
	channelID := cs.MessageChannelID()
	msgBuilder := discord.NewMessageCreate().WithContent(content)
	if len(embeds) > 0 {
		msgBuilder = msgBuilder.WithEmbeds(embeds...)
	}
	if len(components) > 0 {
		msgBuilder = msgBuilder.WithComponents(components...)
	}
	msg, err := cs.Event.Client().Rest.CreateMessage(channelID, msgBuilder)
	if err != nil {
		return
	}
	cs.lastMessageID = msg.ID
}

func (cs *CommandState) UpdateMessage(messageID snowflake.ID, content string, embeds []discord.Embed, components []discord.LayoutComponent) {
	channelID := cs.MessageChannelID()
	updateBuilder := discord.NewMessageUpdate()
	if content != "" {
		updateBuilder.Content = &content
	}
	if len(embeds) > 0 {
		updateBuilder.Embeds = &embeds
	}
	if len(components) > 0 {
		updateBuilder.Components = &components
	}
	cs.Event.Client().Rest.UpdateMessage(channelID, messageID, updateBuilder)
}

func (cs *CommandState) ChannelID() snowflake.ID {
	return cs.MessageChannelID()
}

func (cs *CommandState) LastMessageID() snowflake.ID {
	return cs.lastMessageID
}

type CommandBase struct{}

func (CommandBase) HandleButton(cs *CommandState, customID string) error {
	return nil
}
func (CommandBase) HandleSelectMenu(cs *CommandState, customID string, values []string) error {
	return nil
}
func (CommandBase) HandleModalSubmit(cs *CommandState, customID string, data map[string]string) error {
	return nil
}

type Command interface {
	Name() string
	Description() string
	Execute(cs *CommandState) error
	ParseInteraction(e *events.ApplicationCommandInteractionCreate) *map[string]any
	GetApplicationCommand() discord.ApplicationCommandCreate

	HandleButton(cs *CommandState, customID string) error
	HandleSelectMenu(cs *CommandState, customID string, values []string) error
	HandleModalSubmit(cs *CommandState, customID string, data map[string]string) error
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
