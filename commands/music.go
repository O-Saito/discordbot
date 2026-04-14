package commands

import (
	"context"
	"fmt"
	"mydiscordbot/bot"
	"mydiscordbot/domain"
	"mydiscordbot/services/file"
	"mydiscordbot/services/ytdlp"
	"strings"
	"time"

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
	if cs.G.VoiceConnection == nil {
		cs.SingleRespond("Not connected to a voice channel")
		return nil
	}
	if cs.G.PlaybackControl == nil {
		cs.SingleRespond("Nothing is playing")
		return nil
	}
	cs.G.PlaybackControl <- "pause"
	cs.SingleRespond("Paused")
	return nil
}

func handleResume(cs *bot.CommandState, opts *map[string]any) error {
	if cs.G.VoiceConnection == nil {
		cs.SingleRespond("Not connected to a voice channel")
		return nil
	}
	if cs.G.PlaybackControl == nil {
		cs.SingleRespond("Nothing is playing")
		return nil
	}
	cs.G.PlaybackControl <- "resume"
	cs.SingleRespond("Resumed")
	return nil
}

func handleStop(cs *bot.CommandState, opts *map[string]any) error {
	if cs.G.VoiceConnection == nil {
		cs.SingleRespond("Not connected to a voice channel")
		return nil
	}
	if cs.G.PlaybackControl != nil {
		cs.G.PlaybackControl <- "stop"
	}
	cs.G.Queue.Clear()
	cs.SingleRespond("Stopped and queue cleared")
	return nil
}

func handleSkip(cs *bot.CommandState, opts *map[string]any) error {
	if cs.G.VoiceConnection == nil {
		cs.SingleRespond("Not connected to a voice channel")
		return nil
	}
	if cs.G.PlaybackControl == nil {
		cs.SingleRespond("Nothing is playing")
		return nil
	}
	cs.G.PlaybackControl <- "skip"
	cs.SingleRespond("Skipped")
	return nil
}

func handleQueue(cs *bot.CommandState, opts *map[string]any) error {
	size := cs.G.Queue.Size()
	if size == 0 {
		cs.SingleRespond("Queue is empty")
		return nil
	}

	var msg string
	if cs.G.CurrentTrack != "" {
		msg = fmt.Sprintf("Now playing: %s\n\nQueue (%d):\n", cs.G.CurrentTrack, size)
	} else {
		msg = fmt.Sprintf("Queue (%d):\n", size)
	}

	items := cs.G.Queue.All()
	for i, track := range items {
		msg += fmt.Sprintf("%d. %s\n", i+1, track.Title())
	}

	cs.SingleRespond(msg)
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
	musicFolders := cs.G.Manager.MusicFolders()
	recursive := cs.G.Manager.RecursiveSearch()

	if len(musicFolders) == 0 {
		cs.SingleRespond("No music folders configured")
		return nil
	}

	fileSvc := file.New()
	fileSvc.ListAll(musicFolders, recursive, func(tracks []domain.Track, err error) {
		if err != nil {
			cs.SingleRespond("Failed to list files: " + err.Error())
			return
		}

		if len(tracks) == 0 {
			cs.SingleRespond("No music files found in configured folders")
			return
		}

		var msg string
		if len(tracks) > 20 {
			msg = fmt.Sprintf("Found %d files (showing first 20):\n", len(tracks))
			tracks = tracks[:20]
		} else {
			msg = fmt.Sprintf("Found %d files:\n", len(tracks))
		}

		for i, track := range tracks {
			msg += fmt.Sprintf("%d. %s\n", i+1, track.Title())
		}

		cs.SingleRespond(msg)
	})

	cs.SingleRespond("Loading music files...")
	return nil
}

func handleAutoplay(cs *bot.CommandState, opts *map[string]any) error {
	autoplay, ok := cs.G.Data["autoplay"].(bool)
	if !ok {
		autoplay = true
		cs.G.Data["autoplay"] = true
		cs.SingleRespond("Autoplay: enabled (default)")
		return nil
	}
	cs.G.Data["autoplay"] = !autoplay
	fmt.Printf("[Autoplay] Toggled to: %v for guild %s\n", cs.G.Data["autoplay"], cs.G.GuildId)
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

			ensurePlaybackGoroutine(g)
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

			ensurePlaybackGoroutine(cs.G)
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
			ensurePlaybackGoroutine(g)
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

				ensurePlaybackGoroutine(cs.G)
			})
		})
	})

	return nil
}

func playNext(cs *bot.CommandState) {
	ensurePlaybackGoroutine(cs.G)
}

func ensurePlaybackGoroutine(g *bot.GuildState) {
	if g.PlaybackControl == nil {
		g.PlaybackControl = make(chan string)
		g.PlaybackDone = make(chan struct{})
		go startPlayback(g)
	}
}

func startPlayback(g *bot.GuildState) {
	defer func() {
		g.IsPlaying = false
		g.CurrentTrack = ""
		g.PlaybackControl = nil
		close(g.PlaybackDone)
		g.PlaybackDone = nil
		fmt.Printf("[Playback] Goroutine exiting for guild %s\n", g.GuildId)
	}()

	for {
		autoplay, _ := g.Data["autoplay"].(bool)
		isPaused := !g.Player.IsPlaying()
		queueSize := g.Queue.Size()

		fmt.Printf("[Playback] Loop - isPaused: %v, autoplay: %v, queueSize: %d, isPlaying: %v\n",
			isPaused, autoplay, queueSize, g.IsPlaying)

		if isPaused && g.Queue.IsEmpty() && !autoplay {
			fmt.Printf("[Playback] Exit: isPaused=%v, queueEmpty=%v, autoplay=%v\n", isPaused, g.Queue.IsEmpty(), autoplay)
			g.IsPlaying = false
			notifyPlaybackChanged(g, "stopped")
			return
		}

		if isPaused {
			if g.Queue.IsEmpty() {
				fmt.Printf("[Playback] Queue empty, autoplay=%v\n", autoplay)
				if autoplay {
					notifyPlaybackChanged(g, "autoplay")
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			fmt.Printf("[Playback] Dequeue next track, queueSize before: %d\n", g.Queue.Size())
			track, err := g.Queue.Dequeue()
			if err != nil {
				fmt.Printf("[Playback] Dequeue error: %v\n", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			g.CurrentTrack = track.Title()
			g.IsPlaying = true
			fmt.Printf("[Playback] Playing track: %s\n", track.Title())
			notifyPlaybackChanged(g, "playing")
			g.Player.PlayURLWithSeekAndVC(track.AudioURL(), 48000, 0, g.VoiceConnection)
		}

		fmt.Printf("[Playback] Waiting for Finished or Command...\n")
		select {
		case <-g.Player.Finished():
			fmt.Printf("[Playback] Track finished signal received!\n")
			fmt.Printf("[Playback] After finished - IsPlaying: %v, QueueSize: %d, autoplay: %v\n",
				g.Player.IsPlaying(), g.Queue.Size(), g.Data["autoplay"])
		case cmd := <-g.PlaybackControl:
			fmt.Printf("[Playback] Received command: %s\n", cmd)
			handlePlaybackCommand(g, cmd)
		}
	}
}

func handlePlaybackCommand(g *bot.GuildState, cmd string) {
	switch cmd {
	case "pause":
		g.Player.Pause()
		g.IsPlaying = false
		notifyPlaybackChanged(g, "paused")
	case "resume":
		g.Player.Resume()
		g.IsPlaying = true
		notifyPlaybackChanged(g, "playing")
	case "stop":
		g.Player.Stop()
		g.Queue.Clear()
		g.CurrentTrack = ""
		g.IsPlaying = false
		notifyPlaybackChanged(g, "stopped")
	case "skip":
		g.Player.Stop()
	}
}

func notifyPlaybackChanged(g *bot.GuildState, status string) {
}
