package commands

import (
	"context"
	"fmt"
	"mydiscordbot/bot"
	"mydiscordbot/domain"
	"mydiscordbot/services/file"
	"mydiscordbot/services/ytdlp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var float64zero float64 = 0

type musicHandler func(cs *bot.CommandState, opts *map[string]any) error

var musicHandlers = map[string]musicHandler{
	"play":     handlePlay,
	"pause":    handlePause,
	"resume":   handleResume,
	"stop":     handleStop,
	"skip":     handleSkip,
	"queue":    handleQueue,
	"volume":   handleVolume,
	"list":     handleList,
	"autoplay": handleAutoplay,
	"init":     handleInit,
}

type MusicCommand struct{}

func (c *MusicCommand) Name() string        { return "music" }
func (c *MusicCommand) Description() string { return "Music player commands" }

func (c *MusicCommand) ParseInteraction(i *discordgo.InteractionCreate) *map[string]any {
	subCmd := i.ApplicationCommandData().Options[0]
	result := map[string]any{
		"subcommand": subCmd.Name,
	}
	for _, opt := range subCmd.Options {
		switch opt.Type {
		case discordgo.ApplicationCommandOptionString:
			result[opt.Name] = opt.StringValue()
		case discordgo.ApplicationCommandOptionInteger:
			result[opt.Name] = opt.IntValue()
		case discordgo.ApplicationCommandOptionBoolean:
			result[opt.Name] = opt.BoolValue()
		}
	}
	return &result
}

func (c *MusicCommand) GetApplicationCommand() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        c.Name(),
		Description: c.Description(),
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "play",
				Description: "Play a track or search query",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "query",
						Description: "YouTube URL or search query",
						Required:    true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "pause",
				Description: "Pause the current playback",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "resume",
				Description: "Resume paused playback",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "stop",
				Description: "Stop playback and clear the queue",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "skip",
				Description: "Skip to the next track in queue",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "queue",
				Description: "View the current queue",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "volume",
				Description: "Adjust the playback volume",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "level",
						Description: "Volume level (0-100)",
						Required:    true,
						MinValue:    &float64zero,
						MaxValue:    100,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "list",
				Description: "List all tracks in queue",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "autoplay",
				Description: "Toggle autoplay mode",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "init",
				Description: "Initialize the bot in your voice channel",
			},
		},
	}
}

func (c *MusicCommand) Execute(cs *bot.CommandState) error {
	args := cs.Args
	if args == nil {
		cs.SingleRespond("Command error: no arguments provided")
		return fmt.Errorf("no arguments provided")
	}

	subcommand := (*args)["subcommand"].(string)

	handler, ok := musicHandlers[subcommand]
	if !ok {
		cs.SingleRespond(fmt.Sprintf("Unknown subcommand: %s", subcommand))
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}

	return handler(cs, args)
}

func handlePlay(cs *bot.CommandState, opts *map[string]any) error {
	query, ok := (*opts)["query"].(string)
	if !ok || query == "" {
		cs.SingleRespond("Query parameter is required")
		return fmt.Errorf("usage: /music play <query>")
	}

	if isYouTubeURL(query) {
		return addURL(cs, query)
	}

	return searchAndAdd(cs, query)
}

func handlePause(cs *bot.CommandState, opts *map[string]any) error {
	cs.SingleRespond("Pause functionality not implemented yet")
	return nil
}

func handleResume(cs *bot.CommandState, opts *map[string]any) error {
	cs.SingleRespond("Resume functionality not implemented yet")
	return nil
}

func handleStop(cs *bot.CommandState, opts *map[string]any) error {
	cs.SingleRespond("Stop functionality not implemented yet")
	return nil
}

func handleSkip(cs *bot.CommandState, opts *map[string]any) error {
	cs.SingleRespond("Skip functionality not implemented yet")
	return nil
}

func handleQueue(cs *bot.CommandState, opts *map[string]any) error {
	cs.SingleRespond("Queue functionality not implemented yet")
	return nil
}

func handleVolume(cs *bot.CommandState, opts *map[string]any) error {
	level, hasLevel := (*opts)["level"]

	if !hasLevel {
		vol := cs.G.Player.Volume()
		volPercent := int(vol * 100)
		cs.SingleRespond(fmt.Sprintf("Volume: %d%%", volPercent))
		return nil
	}

	levelInt, ok := level.(int64)
	if !ok {
		cs.SingleRespond("Invalid volume level")
		return nil
	}

	if levelInt < 0 || levelInt > 100 {
		cs.SingleRespond("Volume must be between 0 and 100")
		return nil
	}

	volume := float64(levelInt) / 100.0
	cs.G.Player.SetVolume(volume)
	cs.G.Volume = int(levelInt)
	cs.G.Data["volume"] = int(levelInt)

	cs.SingleRespond(fmt.Sprintf("Volume: %d%%", levelInt))
	return nil
}

func handleList(cs *bot.CommandState, opts *map[string]any) error {
	cs.SingleRespond("List functionality not implemented yet")
	return nil
}

func handleAutoplay(cs *bot.CommandState, opts *map[string]any) error {
	autoplay, ok := cs.G.Data["autoplay"].(bool)
	if !ok {
		autoplay = false
	}
	cs.G.Data["autoplay"] = !autoplay
	if cs.G.Data["autoplay"].(bool) {
		cs.SingleRespond("Autoplay: enabled")
	} else {
		cs.SingleRespond("Autoplay: disabled")
	}
	return nil
}

func handleInit(cs *bot.CommandState, opts *map[string]any) error {
	cs.SingleRespond("Init functionality not implemented yet")
	return nil
}

func autoPlayNext(cs *bot.CommandState) {
	cs.SingleRespond("Autoplay: fetching next track...")
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

func addURL(cs *bot.CommandState, url string) error {
	s := cs.S
	g := cs.G
	if hasPlaylistParam(url) {
		cs.SingleRespond(fmt.Sprintf("A busca %s contem uma playlist adicionando todos...", url))

		ytSvc := ytdlp.New()
		ytSvc.ParsePlaylist(context.Background(), url, func(tracks []domain.Track, err error) {
			if err != nil {
				s.ChannelMessageSend("", "Failed to fetch playlist: "+err.Error())
				return
			}

			s.ChannelMessageSend("", fmt.Sprintf("Adding %d tracks:", len(tracks)))
			for _, track := range tracks {
				err := g.Queue.Enqueue(track)
				if err != nil {
					s.ChannelMessageSend("", "Failed to add to queue: "+err.Error())
					return
				}
			}
			s.ChannelMessageSend("", "Playlist added!")

			if !g.IsPlaying {
				playNext(cs)
			}
		})
		return nil
	}

	url = stripPlaylistParam(url)

	cs.SingleRespond(fmt.Sprintf("Buscando por: %s", url))

	ytSvc := ytdlp.New()
	ytSvc.ParseURL(context.Background(), url, func(track domain.Track, err error) {
		if err != nil {
			cs.SingleRespond(fmt.Sprintf("Falha ao buscar track %s: %s", url, err.Error()))
			return
		}

		ytSvc.GetAudioURL(context.Background(), url, func(audioURL string, err error) {
			if err != nil {
				cs.SingleRespond(fmt.Sprintf("Falha ao buscar audio de %s: %s", url, err.Error()))
				return
			}
			track.SetAudioURL(audioURL)

			err = cs.G.Queue.Enqueue(track)
			if err != nil {
				cs.SingleRespond(fmt.Sprintf("Falha ao adicionar %s na queue: %s", url, err.Error()))
				return
			}

			cs.SingleRespond(fmt.Sprintf("%s adicionado a queue", track.Title()))

			if !cs.G.IsPlaying {
				playNext(cs)
			}
		})
	})

	return nil
}

func searchAndAdd(cs *bot.CommandState, query string) error {
	g := cs.G
	musicFolders := g.Manager.MusicFolders()
	recursive := g.Manager.RecursiveSearch()

	fileSvc := file.New()
	fileSvc.Search(musicFolders, query, recursive, func(fileResults []domain.Track, err error) {
		if err != nil {
			cs.SingleRespond("Falha na busca de arquivos: " + err.Error())
			return
		}

		if len(fileResults) > 0 {
			track := fileResults[0]
			err := g.Queue.Enqueue(track)
			if err != nil {
				cs.SingleRespond("Falha ao adicionar à queue: " + err.Error())
				return
			}

			cs.SingleRespond("Adicionado: " + track.Title())

			if !cs.G.IsPlaying {
				playNext(cs)
			}

			return
		}

		searchAndAddYouTube(cs, query)
	})

	return nil
}

func searchAndAddYouTube(cs *bot.CommandState, query string) error {
	cs.SingleRespond("Buscando no YouTube...")

	ytSvc := ytdlp.New()
	ytSvc.Search(context.Background(), query, 5, func(results []domain.SearchResult, err error) {
		if err != nil {
			cs.SingleRespond("Falha na busca: " + err.Error())
			return
		}

		if len(results) == 0 {
			cs.SingleRespond("Nenhum resultado encontrado")
			return
		}

		result := results[0]
		ytSvc.ParseURL(context.Background(), result.URL, func(track domain.Track, err error) {
			if err != nil {
				cs.SingleRespond("Falha ao analisar URL: " + err.Error())
				return
			}

			ytSvc.GetAudioURL(context.Background(), result.URL, func(audioURL string, err error) {
				if err != nil {
					cs.SingleRespond("Falha ao buscar URL do áudio: " + err.Error())
					return
				}
				track.SetAudioURL(audioURL)

				err = cs.G.Queue.Enqueue(track)
				if err != nil {
					cs.SingleRespond("Falha ao adicionar à queue: " + err.Error())
					return
				}

				cs.SingleRespond("Adicionado: " + track.Title())

				if !cs.G.IsPlaying {
					playNext(cs)
				}
			})
		})
	})

	return nil
}

func playNext(cs *bot.CommandState) {
	if cs.G.IsPlaying {
		return
	}

	if cs.G.Queue.IsEmpty() {
		autoplay, ok := cs.G.Data["autoplay"].(bool)
		if ok && autoplay {
			autoPlayNext(cs)
			return
		}
		cs.G.CurrentTrack = ""
		cs.G.IsPlaying = false
		cs.SingleRespond("Queue está vazia, adicione mais músicas para tocar!")
		return
	}

	cs.G.Player.SetOnFinishedCallback(func() {
		playNext(cs)
	})

	track, err := cs.G.Queue.Dequeue()
	if err != nil {
		cs.G.CurrentTrack = ""
		cs.G.IsPlaying = false
		return
	}

	cs.G.CurrentTrack = track.Title()
	cs.G.IsPlaying = true
	cs.G.Player.PlayURLWithSeekAndVC(track.AudioURL(), 48000, 0, cs.G.VoiceConnection)
	cs.SingleRespond("Tocando: " + track.Title())
}
