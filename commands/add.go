package commands

import (
	"context"
	"fmt"
	"mydiscordbot/bot"
	"strings"

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

func (c *AddCommand) Execute(g *bot.GuildState, s *discordgo.Session, args *map[string]any) error {
	if args == nil || len(*args) == 0 {
		return fmt.Errorf("usage: add <url or search query>")
	}

	arg := (*args)["query"].(string)

	if isYouTubeURL(arg) {
		return addURL(g, s, arg)
	}

	return searchAndAdd(g, s, arg)
}

func isYouTubeURL(input string) bool {
	return strings.Contains(input, "youtube.com") || strings.Contains(input, "youtu.be")
}

func hasPlaylistParam(url string) bool {
	return strings.Contains(url, "list=") || strings.Contains(url, "/playlist")
}

func stripPlaylistParam(url string) string {
	if strings.Contains(url, "list=") {
		parts := strings.Split(url, "list=")
		base := parts[0]
		if idx := strings.Index(base, "?"); idx != -1 {
			base = base[:idx]
		}
		if len(parts) > 1 {
			rest := parts[1]
			if ampIdx := strings.Index(rest, "&"); ampIdx != -1 {
				rest = rest[ampIdx+1:]
				if len(rest) > 0 {
					if !strings.HasSuffix(base, "?") {
						base += "?"
					}
					base += rest
				}
			}
		}
		if strings.HasSuffix(base, "?") {
			base = strings.TrimSuffix(base, "?")
		}
		return base
	}
	return url
}

func addURL(g *bot.GuildState, s *discordgo.Session, url string) error {
	if hasPlaylistParam(url) {
		s.ChannelMessageSend("", "This URL contains a playlist, adding all tracks...")

		tracks, err := g.YouTubeService.ParsePlaylist(context.Background(), url)
		if err != nil {
			return fmt.Errorf("failed to fetch playlist: %w", err)
		}

		s.ChannelMessageSend("", fmt.Sprintf("Adding %d tracks:", len(tracks)))
		for _, track := range tracks {
			err := g.Queue.Enqueue(track)
			if err != nil {
				return fmt.Errorf("failed to add to queue: %w", err)
			}
		}
		s.ChannelMessageSend("", "Playlist added!")

		if !g.IsPlaying {
			playNext(g, s)
		}
		return nil
	}

	url = stripPlaylistParam(url)

	s.ChannelMessageSend("", "Fetching track...")

	track, err := g.YouTubeService.ParseURL(context.Background(), url)
	if err != nil {
		return fmt.Errorf("failed to fetch track: %w", err)
	}

	audioURL, err := g.YouTubeService.GetAudioURL(context.Background(), url)
	if err != nil {
		return fmt.Errorf("failed to get audio URL: %w", err)
	}
	track.SetAudioURL(audioURL)

	err = g.Queue.Enqueue(track)
	if err != nil {
		return fmt.Errorf("failed to add to queue: %w", err)
	}

	s.ChannelMessageSend("", "Added: "+track.Title())

	if !g.IsPlaying {
		playNext(g, s)
	}

	return nil
}

func searchAndAdd(g *bot.GuildState, s *discordgo.Session, query string) error {
	musicFolders := g.Manager.MusicFolders()
	recursive := g.Manager.RecursiveSearch()

	fileResults, err := g.FileService.Search(musicFolders, query, recursive)
	if err != nil {
		return fmt.Errorf("file search failed: %w", err)
	}

	if len(fileResults) > 0 {
		track := fileResults[0]
		err := g.Queue.Enqueue(track)
		if err != nil {
			return fmt.Errorf("failed to add to queue: %w", err)
		}

		s.ChannelMessageSend("", "Added: "+track.Title())

		if !g.IsPlaying {
			playNext(g, s)
		}

		return nil
	}

	return searchAndAddYouTube(g, s, query)
}

func searchAndAddYouTube(g *bot.GuildState, s *discordgo.Session, query string) error {
	s.ChannelMessageSend("", "Searching YouTube...")

	results, err := g.YouTubeService.Search(context.Background(), query, 5)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		s.ChannelMessageSend("", "No results found")
		return nil
	}

	result := results[0]
	track, err := g.YouTubeService.ParseURL(context.Background(), result.URL)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	audioURL, err := g.YouTubeService.GetAudioURL(context.Background(), result.URL)
	if err != nil {
		return fmt.Errorf("failed to get audio URL: %w", err)
	}
	track.SetAudioURL(audioURL)

	err = g.Queue.Enqueue(track)
	if err != nil {
		return fmt.Errorf("failed to add to queue: %w", err)
	}

	s.ChannelMessageSend("", "Added: "+track.Title())

	if !g.IsPlaying {
		playNext(g, s)
	}

	return nil
}

func playNext(g *bot.GuildState, s *discordgo.Session) {
	if g.Queue.IsEmpty() {
		g.CurrentTrack = ""
		g.IsPlaying = false
		s.ChannelMessageSend("", "Queue is empty")
		return
	}

	g.Player.SetOnFinishedCallback(func() {
		playNext(g, s)
	})

	track, err := g.Queue.Dequeue()
	if err != nil {
		g.CurrentTrack = ""
		g.IsPlaying = false
		return
	}

	g.CurrentTrack = track.Title()
	g.IsPlaying = true
	g.Player.PlayURLWithSeekAndVC(track.AudioURL(), 48000, 0, g.VoiceConnection)
	s.ChannelMessageSend("", "Playing: "+track.Title())
}
