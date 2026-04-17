package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mydiscordbot/bot"
	"mydiscordbot/discord_helper"
	"mydiscordbot/domain"
	"mydiscordbot/services/file"
	"mydiscordbot/services/ytdlp"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
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
}

type MusicCommand struct {
	bot.CommandBase
}

func (c *MusicCommand) Name() string        { return "music" }
func (c *MusicCommand) Description() string { return "Music player commands" }

func (c *MusicCommand) HandleButton(cs *bot.CommandState, customID string) error {
	musicFolders := cs.G.Manager.MusicFolders()
	recursive := cs.G.Manager.RecursiveSearch()

	queuePage, queueOk := discord_helper.ParseQueueAction(customID)
	if queueOk {
		messageID := cs.LastMessageID()

		queueTracks := cs.G.Queue.All()
		currentTrack := cs.G.CurrentTrack

		totalPages := (len(queueTracks) + 9) / 10
		if totalPages == 0 {
			totalPages = 1
		}

		msg, components := discord_helper.BuildQueuePageComponents(queueTracks, currentTrack, queuePage, totalPages)

		cs.UpdateMessage(messageID, "", []discord.Embed{msg}, components)
		return nil
	}

	page, ok := discord_helper.ParseListPageAction(customID)
	if !ok {
		fmt.Printf("[Music HandleButton] Failed to parse customID=%s\n", customID)
		return nil
	}

	fileSvc := file.New()
	fileSvc.ListAll(musicFolders, recursive, func(tracks []domain.Track, err error) {
		if err != nil {
			fmt.Printf("[Music HandleButton] ListAll error: %v\n", err)
			return
		}

		totalPages := (len(tracks) + 9) / 10
		if totalPages == 0 {
			totalPages = 1
		}

		messageID := cs.LastMessageID()

		embed, components := discord_helper.BuildListPageComponents(tracks, page, totalPages)

		cs.UpdateMessage(messageID, "", []discord.Embed{embed}, components)
	})

	return nil
}

const TrackBlue = 0x3498db

func buildButtons(buttonIDs []string) []discord.LayoutComponent {
	if len(buttonIDs) == 0 {
		return nil
	}

	var buttons []discord.InteractiveComponent
	for _, id := range buttonIDs {
		parts := strings.Split(id, "_")
		if len(parts) < 3 {
			continue
		}

		action := parts[1]
		label := "◀"
		if action == "next" {
			label = "▶"
		}

		buttons = append(buttons, discord.NewSecondaryButton(label, id))
	}

	if len(buttons) == 0 {
		return nil
	}

	return []discord.LayoutComponent{discord.NewActionRow(buttons...)}
}

func (c *MusicCommand) HandleSelectMenu(cs *bot.CommandState, customID string, values []string) error {
	musicFolders := cs.G.Manager.MusicFolders()
	recursive := cs.G.Manager.RecursiveSearch()

	selectedValue := ""
	if len(values) > 0 {
		selectedValue = values[0]
	}

	page, trackIndex, ok := discord_helper.ParseListSelectAction(selectedValue)
	if !ok {
		fmt.Printf("[Music HandleSelectMenu] Failed to parse customID=%s\n", selectedValue)
		return nil
	}

	fmt.Printf("[Music HandleSelectMenu] page=%d, trackIndex=%d\n", page, trackIndex)

	fileSvc := file.New()
	fileSvc.ListAll(musicFolders, recursive, func(tracks []domain.Track, err error) {
		if err != nil {
			cs.SingleResponse("Error listing files: " + err.Error())
			return
		}

		globalIndex := page*10 + trackIndex
		if globalIndex >= len(tracks) {
			cs.SingleResponse("Track not found")
			return
		}

		track := tracks[globalIndex]
		queueErr := cs.G.Queue.Enqueue(track)
		if queueErr != nil {
			cs.SingleResponse("Failed to add to queue: " + queueErr.Error())
			return
		}

		ensurePlaybackGoroutine(cs)

		embed := discord.NewEmbed().
			WithTitle("Added to queue").
			WithDescription(track.Title()).
			WithColor(0x2ecc71)

		cs.SendMessage("", []discord.Embed{embed}, nil)
	})

	return nil
}

func (c *MusicCommand) ParseInteraction(e *events.ApplicationCommandInteractionCreate) *map[string]any {
	data := e.SlashCommandInteractionData()

	subCmd := strings.TrimPrefix(data.CommandPath(), "/music/")

	result := map[string]any{
		"subcommand": subCmd,
	}

	if member := e.Member(); member != nil {
		result["member"] = member.User.ID
	}

	for name, opt := range data.Options {
		switch opt.Type {
		case discord.ApplicationCommandOptionTypeString:
			result[name] = opt.String()
		case discord.ApplicationCommandOptionTypeInt:
			result[name] = int(opt.Int())
		case discord.ApplicationCommandOptionTypeBool:
			result[name] = opt.Bool()
		case discord.ApplicationCommandOptionTypeFloat:
			result[name] = opt.Float()
		}
	}

	return &result
}

func (c *MusicCommand) GetApplicationCommand() discord.ApplicationCommandCreate {
	return discord.SlashCommandCreate{
		Name:        "music",
		Description: "Music player",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "play",
				Description: "Play a track or search query",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "query",
						Description: "YouTube URL or search query",
						Required:    true,
					},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "pause",
				Description: "Pause the current playback",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "resume",
				Description: "Resume paused playback",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "stop",
				Description: "Stop playback and clear the queue",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "skip",
				Description: "Skip to the next track in queue",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "queue",
				Description: "View the current queue",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "volume",
				Description: "Adjust the playback volume",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionInt{
						Name:        "level",
						Description: "Volume level (0-100)",
						Required:    true,
						MinValue:    newInt(0),
						MaxValue:    newInt(100),
					},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "list",
				Description: "List all tracks in queue",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "autoplay",
				Description: "Toggle autoplay mode",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "init",
				Description: "Initialize the bot in your voice channel",
			},
		},
	}
}

func newInt(i int) *int {
	return &i
}

func (c *MusicCommand) Execute(cs *bot.CommandState) error {
	args := cs.Args
	if args == nil {
		cs.SingleResponse("Command error: no arguments provided")
		return fmt.Errorf("no arguments provided")
	}

	subcommand := (*args)["subcommand"].(string)

	handler, ok := musicHandlers[subcommand]
	if !ok {
		cs.SingleResponse(fmt.Sprintf("Unknown subcommand: %s", subcommand))
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}

	return handler(cs, args)
}

func handlePlay(cs *bot.CommandState, opts *map[string]any) error {
	query, ok := (*opts)["query"].(string)
	if !ok || query == "" {
		cs.SingleResponse("Query parameter is required")
		return fmt.Errorf("usage: /music play <query>")
	}

	if isYouTubeURL(query) {
		return addURL(cs, query)
	}

	return searchAndAdd(cs, query)
}

func handlePause(cs *bot.CommandState, opts *map[string]any) error {
	if cs.G.VoiceConn == nil {
		cs.SingleResponse("Not connected to a voice channel")
		return nil
	}
	if cs.G.PlaybackControl == nil {
		cs.SingleResponse("Nothing is playing")
		return nil
	}
	cs.G.PlaybackControl <- "pause"
	cs.SingleResponse("Paused")
	return nil
}

func handleResume(cs *bot.CommandState, opts *map[string]any) error {
	if cs.G.VoiceConn == nil {
		cs.SingleResponse("Not connected to a voice channel")
		return nil
	}
	if cs.G.PlaybackControl == nil {
		cs.SingleResponse("Nothing is playing")
		return nil
	}
	cs.G.PlaybackControl <- "resume"
	cs.SingleResponse("Resumed")
	return nil
}

func handleStop(cs *bot.CommandState, opts *map[string]any) error {
	if cs.G.VoiceConn == nil {
		cs.SingleResponse("Not connected to a voice channel")
		return nil
	}
	if cs.G.PlaybackControl != nil {
		cs.G.PlaybackControl <- "stop"
	}
	cs.G.Queue.Clear()
	cs.SingleResponse("Stopped and queue cleared")
	return nil
}

func handleSkip(cs *bot.CommandState, opts *map[string]any) error {
	if cs.G.VoiceConn == nil {
		cs.SingleResponse("Not connected to a voice channel")
		return nil
	}
	if cs.G.PlaybackControl == nil {
		cs.SingleResponse("Nothing is playing")
		return nil
	}
	cs.G.PlaybackControl <- "skip"
	cs.SingleResponse("Skipped")
	return nil
}

func handleQueue(cs *bot.CommandState, opts *map[string]any) error {
	queueTracks := cs.G.Queue.All()
	currentTrack := cs.G.CurrentTrack

	totalPages := (len(queueTracks) + 9) / 10
	if totalPages == 0 {
		totalPages = 1
	}

	embed, components := discord_helper.BuildQueuePageComponents(queueTracks, currentTrack, 0, totalPages)

	cs.SendMessage("", []discord.Embed{embed}, components)

	return nil
}

func handleVolume(cs *bot.CommandState, opts *map[string]any) error {
	level, hasLevel := (*opts)["level"]

	if !hasLevel {
		vol := cs.G.Player.Volume()
		volPercent := int(vol * 100)
		cs.SingleResponse(fmt.Sprintf("Volume: %d%%", volPercent))
		return nil
	}

	levelInt, ok := level.(int64)
	if !ok {
		cs.SingleResponse("Invalid volume level")
		return nil
	}

	if levelInt < 0 || levelInt > 100 {
		cs.SingleResponse("Volume must be between 0 and 100")
		return nil
	}

	volume := float64(levelInt) / 100.0
	cs.G.Player.SetVolume(volume)
	cs.G.Volume = int(levelInt)
	cs.G.Data["volume"] = int(levelInt)

	cs.SingleResponse(fmt.Sprintf("Volume: %d%%", levelInt))
	return nil
}

func handleList(cs *bot.CommandState, opts *map[string]any) error {
	musicFolders := cs.G.Manager.MusicFolders()
	recursive := cs.G.Manager.RecursiveSearch()

	if len(musicFolders) == 0 {
		cs.SingleResponse("No music folders configured")
		return nil
	}

	fileSvc := file.New()
	fileSvc.ListAll(musicFolders, recursive, func(tracks []domain.Track, err error) {
		if err != nil {
			cs.SingleResponse("Failed to list files: " + err.Error())
			return
		}

		if len(tracks) == 0 {
			cs.SingleResponse("No music files found in configured folders")
			return
		}

		totalPages := (len(tracks) + 9) / 10
		if totalPages == 0 {
			totalPages = 1
		}

		embed, components := discord_helper.BuildListPageComponents(tracks, 0, totalPages)

		cs.SingleResponseWithEmbedComponents(" ", []discord.Embed{embed}, components)
	})

	cs.SingleResponse("Loading music files...")
	return nil
}

func handleAutoplay(cs *bot.CommandState, opts *map[string]any) error {
	autoplay, ok := cs.G.Data["autoplay"].(bool)
	if !ok {
		autoplay = true
		cs.G.Data["autoplay"] = true
		cs.SingleResponse("Autoplay: enabled (default)")
		return nil
	}
	cs.G.Data["autoplay"] = !autoplay
	fmt.Printf("[Autoplay] Toggled to: %v for guild %s\n", cs.G.Data["autoplay"], cs.G.GuildId)
	if cs.G.Data["autoplay"].(bool) {
		cs.SingleResponse("Autoplay: enabled")
	} else {
		cs.SingleResponse("Autoplay: disabled")
	}
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
	g := cs.G
	if hasPlaylistParam(url) {
		cs.SingleResponse(fmt.Sprintf("A busca %s contem uma playlist adicionando todos...", url))

		ytSvc := ytdlp.New()
		ytSvc.ParsePlaylist(context.Background(), url, func(tracks []domain.Track, err error) {
			if err != nil {
				cs.SingleResponse("Failed to fetch playlist: " + err.Error())
				return
			}

			cs.SingleResponse(fmt.Sprintf("Adding %d tracks:", len(tracks)))
			for _, track := range tracks {
				err := g.Queue.Enqueue(track)
				if err != nil {
					cs.SingleResponse("Failed to add to queue: " + err.Error())
					return
				}
			}
			cs.SingleResponse("Playlist added!")

			ensurePlaybackGoroutine(cs)
		})
		return nil
	}

	url = stripPlaylistParam(url)

	cs.SingleResponse(fmt.Sprintf("Buscando por: %s", url))

	ytSvc := ytdlp.New()
	ytSvc.ParseURL(context.Background(), url, func(track domain.Track, err error) {
		if err != nil {
			cs.SingleResponse(fmt.Sprintf("Falha ao buscar track %s: %s", url, err.Error()))
			return
		}

		ytSvc.GetAudioURL(context.Background(), url, func(audioURL string, err error) {
			if err != nil {
				cs.SingleResponse(fmt.Sprintf("Falha ao buscar audio de %s: %s", url, err.Error()))
				return
			}
			track.SetAudioURL(audioURL)

			err = cs.G.Queue.Enqueue(track)
			if err != nil {
				cs.SingleResponse(fmt.Sprintf("Falha ao adicionar %s na queue: %s", url, err.Error()))
				return
			}

			cs.SingleResponse(fmt.Sprintf("%s adicionado a queue", track.Title()))

			ensurePlaybackGoroutine(cs)
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
			cs.SingleResponse("Falha na busca de arquivos: " + err.Error())
			return
		}

		if len(fileResults) > 0 {
			track := fileResults[0]
			err := g.Queue.Enqueue(track)
			if err != nil {
				cs.SingleResponse("Falha ao adicionar à queue: " + err.Error())
				return
			}

			cs.SingleResponse("Adicionado: " + track.Title())
			ensurePlaybackGoroutine(cs)
			return
		}

		searchAndAddYouTube(cs, query)
	})

	return nil
}

func searchAndAddYouTube(cs *bot.CommandState, query string) error {
	cs.SingleResponse("Buscando no YouTube...")

	ytSvc := ytdlp.New()
	ytSvc.Search(context.Background(), query, 5, func(results []domain.SearchResult, err error) {
		if err != nil {
			cs.SingleResponse("Falha na busca: " + err.Error())
			return
		}

		if len(results) == 0 {
			cs.SingleResponse("Nenhum resultado encontrado")
			return
		}

		result := results[0]
		ytSvc.ParseURL(context.Background(), result.URL, func(track domain.Track, err error) {
			if err != nil {
				cs.SingleResponse("Falha ao analisar URL: " + err.Error())
				return
			}

			ytSvc.GetAudioURL(context.Background(), result.URL, func(audioURL string, err error) {
				if err != nil {
					cs.SingleResponse("Falha ao buscar URL do áudio: " + err.Error())
					return
				}
				track.SetAudioURL(audioURL)

				err = cs.G.Queue.Enqueue(track)
				if err != nil {
					cs.SingleResponse("Falha ao adicionar à queue: " + err.Error())
					return
				}

				cs.SingleResponse("Adicionado: " + track.Title())

				ensurePlaybackGoroutine(cs)
			})
		})
	})

	return nil
}

func ensurePlaybackGoroutine(cs *bot.CommandState) {
	g := cs.G

	if g.VoiceConn == nil {
		if err := JoinVoiceChannel(cs); err != nil {
			cs.SingleResponse(err.Error())
			return
		}
	}

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

	ctx := context.Background()

	for {
		autoplay, _ := g.Data["autoplay"].(bool)
		isPaused := !g.Player.IsPlaying()
		queueSize := g.Queue.Size()

		fmt.Printf("[Playback] Loop - isPaused: %v, autoplay: %v, queueSize: %d, isPlaying: %v\n",
			isPaused, autoplay, queueSize, g.IsPlaying)

		if isPaused && g.Queue.IsEmpty() && !autoplay {
			fmt.Printf("[Playback] Exit: isPaused=%v, queueEmpty=%v, autoplay=%v\n", isPaused, g.Queue.IsEmpty(), autoplay)
			g.IsPlaying = false
			return
		}

		if isPaused {
			if g.Queue.IsEmpty() {
				fmt.Printf("[Playback] Queue empty, autoplay=%v\n", autoplay)
				if autoplay {
					fmt.Printf("[Playback] Autoplay enabled but no next track\n")
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
			g.Player.SetVoiceConn(g.VoiceConn)
			audioInput := g.OpenAudioStream()
			g.Player.PlayURLWithSeekAndVC(ctx, track.AudioURL(), 48000, 0, g.VoiceConn, audioInput)
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
	case "resume":
		g.Player.Resume()
		g.IsPlaying = true
	case "stop":
		g.Player.Stop()
		g.Queue.Clear()
		g.CurrentTrack = ""
		g.IsPlaying = false
	case "skip":
		g.Player.Stop()
	}
}

func (c *MusicCommand) HandleModalSubmit(cs *bot.CommandState, customID string, data map[string]string) error {
	return nil
}
