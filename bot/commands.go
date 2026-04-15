package bot

import (
	"sort"
	"sync"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

type CommandState struct {
	G             *GuildState
	Client        *bot.Client
	Args          *map[string]any
	channelID     snowflake.ID
	lastMessageID snowflake.ID
	mu            sync.RWMutex
}

func (cs *CommandState) MessageChannelID() snowflake.ID {
	return cs.channelID
}

func (cs *CommandState) GuildID() snowflake.ID {
	if cs.lastMessageID == 0 {
		return 0
	}
	return snowflake.MustParse(cs.G.GuildId)
}

func (cs *CommandState) Member() *discord.ResolvedMember {
	return nil
}

func (cs *CommandState) SingleResponse(content string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.lastMessageID != 0 {
		cs.Client.Rest.UpdateMessage(cs.channelID, cs.lastMessageID,
			discord.NewMessageUpdate().WithContent(content))
		return
	}

	msg, err := cs.Client.Rest.CreateMessage(cs.channelID, discord.NewMessageCreate().WithContent(content))
	if err != nil {
		return
	}
	cs.lastMessageID = msg.ID
}

func (cs *CommandState) SingleResponseEphemeral(content string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.lastMessageID != 0 {
		cs.Client.Rest.UpdateMessage(cs.channelID, cs.lastMessageID,
			discord.NewMessageUpdate().WithContent(content).WithFlags(discord.MessageFlagEphemeral))
		return
	}
	msg, _ := cs.Client.Rest.CreateMessage(cs.channelID, discord.NewMessageCreate().WithContent(content).WithFlags(discord.MessageFlagEphemeral))
	cs.lastMessageID = msg.ID
}

func (cs *CommandState) SingleResponseWithEmbed(content string, embed []discord.Embed) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.lastMessageID != 0 {
		cs.Client.Rest.UpdateMessage(cs.channelID, cs.lastMessageID,
			discord.NewMessageUpdate().WithContent(content).WithEmbeds(embed...))
		return
	}

	msg, _ := cs.Client.Rest.CreateMessage(cs.channelID, discord.NewMessageCreate().WithContent(content).WithEmbeds(embed...))
	cs.lastMessageID = msg.ID
}

func (cs *CommandState) SingleResponseWithEmbedComponents(content string, embed []discord.Embed, components []discord.LayoutComponent) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.lastMessageID != 0 {
		cs.Client.Rest.UpdateMessage(cs.channelID, cs.lastMessageID,
			discord.NewMessageUpdate().WithContent(content).WithEmbeds(embed...).WithComponents(components...))
		return
	}

	msg, _ := cs.Client.Rest.CreateMessage(cs.channelID, discord.NewMessageCreate().
		WithContent(content).WithEmbeds(embed...).WithComponents(components...))
	cs.lastMessageID = msg.ID
}

func (cs *CommandState) SingleResponseWithComponents(content string, components []discord.LayoutComponent) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.lastMessageID != 0 {
		cs.Client.Rest.UpdateMessage(cs.channelID, cs.lastMessageID,
			discord.NewMessageUpdate().WithContent(content).WithComponents(components...))
		return
	}

	msg, _ := cs.Client.Rest.CreateMessage(cs.channelID, discord.NewMessageCreate().
		WithContent(content).WithComponents(components...))
	cs.lastMessageID = msg.ID
}

func (cs *CommandState) SendEmbed(embed discord.Embed) {
	cs.Client.Rest.CreateMessage(cs.channelID, discord.NewMessageCreate().WithEmbeds(embed))
}

func (cs *CommandState) SendMessage(content string, embeds []discord.Embed, components []discord.LayoutComponent) {
	msgBuilder := discord.NewMessageCreate().WithContent(content)
	if len(embeds) > 0 {
		msgBuilder = msgBuilder.WithEmbeds(embeds...)
	}
	if len(components) > 0 {
		msgBuilder = msgBuilder.WithComponents(components...)
	}
	msg, err := cs.Client.Rest.CreateMessage(cs.channelID, msgBuilder)
	if err != nil {
		return
	}

	cs.mu.Lock()
	cs.lastMessageID = msg.ID
	cs.mu.Unlock()
}

func (cs *CommandState) UpdateMessage(messageID snowflake.ID, content string, embeds []discord.Embed, components []discord.LayoutComponent) {
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
	cs.Client.Rest.UpdateMessage(cs.channelID, messageID, updateBuilder)
}

func (cs *CommandState) ChannelID() snowflake.ID {
	return cs.channelID
}

func (cs *CommandState) LastMessageID() snowflake.ID {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
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
