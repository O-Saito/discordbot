package discord_helper

import (
	"fmt"
	"strconv"
	"strings"

	"mydiscordbot/domain"

	"github.com/bwmarrin/discordgo"
)

const (
	TrackBlue     = 0x3498db
	TracksPerPage = 10
	MusicPrefix   = "music"
)

func BuildListPageComponents(tracks []domain.Track, page, totalPages int) (*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	start := page * TracksPerPage
	end := start + TracksPerPage
	if end > len(tracks) {
		end = len(tracks)
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🎵 Music Library",
		Description: fmt.Sprintf("Page %d of %d (Total: %d)", page+1, totalPages, len(tracks)),
		Color:       TrackBlue,
		Fields:      []*discordgo.MessageEmbedField{},
	}

	for idx := start; idx < end; idx++ {
		title := tracks[idx].Title()
		if len(title) > 100 {
			title = title[:97] + "..."
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%d. %s", idx+1, title),
			Value:  "🎵",
			Inline: false,
		})
	}

	var selectOptions []discordgo.SelectMenuOption
	for idx := start; idx < end; idx++ {
		title := tracks[idx].Title()
		if len(title) > 100 {
			title = title[:97] + "..."
		}
		selectOptions = append(selectOptions, discordgo.SelectMenuOption{
			Label: fmt.Sprintf("%d. %s", idx+1, title),
			Value: fmt.Sprintf("%s_select_%d_%d", MusicPrefix, page, idx),
		})
	}

	minVals := 1
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    fmt.Sprintf("%s_select_%d", MusicPrefix, page),
					MinValues:   &minVals,
					MaxValues:   1,
					Options:     selectOptions,
					Placeholder: "Select a track to add to queue...",
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "◀",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("%s_list_prev_%d", MusicPrefix, page),
					Disabled: page == 0,
				},
				discordgo.Button{
					Label:    "▶",
					Style:    discordgo.SecondaryButton,
					CustomID: fmt.Sprintf("%s_list_next_%d", MusicPrefix, page),
					Disabled: page >= totalPages-1,
				},
			},
		},
	}

	return embed, components
}

func ParseListPageAction(customID string) (page int, ok bool) {
	fmt.Printf("[ParseListPageAction] Input: customID=%s\n", customID)

	if !strings.HasPrefix(customID, "list_") {
		fmt.Printf("[ParseListPageAction] FAILED: no list_ prefix\n")
		return 0, false
	}
	fmt.Printf("[ParseListPageAction] Prefix matched\n")

	action := strings.TrimPrefix(customID, "list_")
	fmt.Printf("[ParseListPageAction] Action after trim: %s\n", action)

	if strings.HasPrefix(action, "prev_") {
		p, err := strconv.Atoi(strings.TrimPrefix(action, "prev_"))
		if err != nil {
			fmt.Printf("[ParseListPageAction] Atoi error: %v\n", err)
			return 0, false
		}
		fmt.Printf("[ParseListPageAction] prev: p=%d, newPage=%d\n", p, p-1)
		return p - 1, true
	}

	if strings.HasPrefix(action, "next_") {
		p, err := strconv.Atoi(strings.TrimPrefix(action, "next_"))
		if err != nil {
			fmt.Printf("[ParseListPageAction] Atoi error: %v\n", err)
			return 0, false
		}
		fmt.Printf("[ParseListPageAction] next: p=%d, newPage=%d\n", p, p+1)
		return p + 1, true
	}

	fmt.Printf("[ParseListPageAction] FAILED: no prev_ or next_ prefix\n")
	return 0, false
}

func ParseListPlayAction(customID string) (page, trackIndex int, ok bool) {
	if !strings.HasPrefix(customID, "play_") {
		return 0, 0, false
	}

	parts := strings.TrimPrefix(customID, "play_")
	partsSlice := strings.Split(parts, "_")
	if len(partsSlice) != 2 {
		return 0, 0, false
	}

	page, err := strconv.Atoi(partsSlice[0])
	if err != nil {
		return 0, 0, false
	}

	trackIdx, err := strconv.Atoi(partsSlice[1])
	if err != nil {
		return 0, 0, false
	}

	fmt.Printf("[ParseListPlayAction] page=%d, trackIndex=%d\n", page, trackIdx)
	return page, trackIdx, true
}

func ParseListSelectAction(customID string) (page, trackIndex int, ok bool) {
	if !strings.HasPrefix(customID, "music_select_") {
		return 0, 0, false
	}

	parts := strings.TrimPrefix(customID, "music_select_")
	partsSlice := strings.Split(parts, "_")
	if len(partsSlice) != 2 {
		return 0, 0, false
	}

	page, err := strconv.Atoi(partsSlice[0])
	if err != nil {
		return 0, 0, false
	}

	trackIdx, err := strconv.Atoi(partsSlice[1])
	if err != nil {
		return 0, 0, false
	}

	fmt.Printf("[ParseListSelectAction] page=%d, trackIndex=%d\n", page, trackIdx)
	return page, trackIdx, true
}
