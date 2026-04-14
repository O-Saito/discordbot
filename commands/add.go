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

func (c *AddCommand) Execute(cs *bot.CommandState) error {
	args := cs.Args
	if args == nil || len(*args) == 0 {
		cs.SingleRespond("Parametro query não informado!")
		return fmt.Errorf("usage: add <url or search query>")
	}

	arg := (*args)["query"].(string)

	if isYouTubeURL(arg) {
		return addURL(cs, arg)
	}

	return searchAndAdd(cs, arg)
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
	if cs.G.Queue.IsEmpty() {
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
