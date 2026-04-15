package discord_helper

import (
	"fmt"
	"strconv"
	"strings"

	"mydiscordbot/domain"

	"github.com/disgoorg/disgo/discord"
)

const (
	TrackBlue     = 0x3498db
	TracksPerPage = 10
	MusicPrefix   = "music"
)

func BuildListPageComponents(tracks []domain.Track, page, totalPages int) (discord.Embed, []discord.LayoutComponent) {
	start := page * TracksPerPage
	end := start + TracksPerPage
	if end > len(tracks) {
		end = len(tracks)
	}

	embed := discord.NewEmbed().
		WithTitle("🎵 Music Library").
		WithDescription(fmt.Sprintf("Page %d of %d (Total: %d)", page+1, totalPages, len(tracks))).
		WithColor(TrackBlue)

	fields := make([]discord.EmbedField, 0, end-start)
	for idx := start; idx < end; idx++ {
		title := tracks[idx].Title()
		if len(title) > 100 {
			title = title[:97] + "..."
		}
		inline := false
		fields = append(fields, discord.EmbedField{
			Name:   fmt.Sprintf("%d. %s", idx+1, title),
			Value:  "",
			Inline: &inline,
		})
	}
	embed = embed.WithFields(fields...)

	var selectOptions []discord.StringSelectMenuOption
	for idx := start; idx < end; idx++ {
		title := tracks[idx].Title()
		if len(title) > 100 {
			title = title[:97] + "..."
		}
		selectOptions = append(selectOptions, discord.NewStringSelectMenuOption(
			fmt.Sprintf("%d. %s", idx+1, title),
			fmt.Sprintf("%s_select_%d_%d", MusicPrefix, page, idx),
		))
	}

	var components []discord.LayoutComponent

	components = append(components, discord.NewActionRow(
		discord.NewStringSelectMenu(
			fmt.Sprintf("%s_select_%d", MusicPrefix, page),
			"Select a track to add to queue...",
			selectOptions...,
		).WithMinValues(1).WithMaxValues(1),
	))

	if totalPages > 1 {
		prevButton := discord.NewSecondaryButton("◀", fmt.Sprintf("%s_list_prev_%d", MusicPrefix, page))
		if page == 0 {
			prevButton = prevButton.AsDisabled()
		}
		nextButton := discord.NewSecondaryButton("▶", fmt.Sprintf("%s_list_next_%d", MusicPrefix, page))
		if page >= totalPages-1 {
			nextButton = nextButton.AsDisabled()
		}
		components = append(components, discord.NewActionRow(prevButton, nextButton))
	}

	return embed, components
}

func ParseListPageAction(customID string) (page int, ok bool) {
	if !strings.HasPrefix(customID, "list_") {
		return 0, false
	}

	action := strings.TrimPrefix(customID, "list_")

	if strings.HasPrefix(action, "prev_") {
		p, err := strconv.Atoi(strings.TrimPrefix(action, "prev_"))
		if err != nil {
			return 0, false
		}
		return p - 1, true
	}

	if strings.HasPrefix(action, "next_") {
		p, err := strconv.Atoi(strings.TrimPrefix(action, "next_"))
		if err != nil {
			return 0, false
		}
		return p + 1, true
	}

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

	return page, trackIdx, true
}

func ParseQueueAction(customID string) (page int, ok bool) {
	if !strings.HasPrefix(customID, "queue_") {
		return 0, false
	}

	action := strings.TrimPrefix(customID, "queue_")

	if strings.HasPrefix(action, "prev_") {
		p, err := strconv.Atoi(strings.TrimPrefix(action, "prev_"))
		if err != nil {
			return 0, false
		}
		return p - 1, true
	}

	if strings.HasPrefix(action, "next_") {
		p, err := strconv.Atoi(strings.TrimPrefix(action, "next_"))
		if err != nil {
			return 0, false
		}
		return p + 1, true
	}

	return 0, false
}

func BuildQueuePageComponents(tracks []domain.Track, currentTrack string, page, totalPages int) (discord.Embed, []discord.LayoutComponent) {
	start := page * TracksPerPage
	end := start + TracksPerPage
	if end > len(tracks) {
		end = len(tracks)
	}

	fields := make([]discord.EmbedField, 0, end-start+2)
	inlineFalse := false

	if currentTrack != "" {
		fields = append(fields, discord.EmbedField{
			Name:   "Now Playing",
			Value:  currentTrack,
			Inline: &inlineFalse,
		})
	}

	fields = append(fields, discord.EmbedField{
		Name:   "Up Next",
		Value:  "─────────────────────",
		Inline: &inlineFalse,
	})

	for i := start; i < end && i < len(tracks); i++ {
		trackNum := i + 1
		title := tracks[i].Title()
		if len(title) > 100 {
			title = title[:97] + "..."
		}
		fields = append(fields, discord.EmbedField{
			Name:   fmt.Sprintf("%d. %s", trackNum, title),
			Value:  "",
			Inline: &inlineFalse,
		})
	}

	embed := discord.NewEmbed().
		WithTitle("🎵 Music Queue").
		WithDescription(fmt.Sprintf("Page %d of %d", page+1, totalPages)).
		WithColor(TrackBlue).
		WithFields(fields...)

	var components []discord.LayoutComponent
	if totalPages > 1 {
		prevButton := discord.NewSecondaryButton("◀", fmt.Sprintf("queue_prev_%d", page))
		if page == 0 {
			prevButton = prevButton.AsDisabled()
		}
		nextButton := discord.NewSecondaryButton("▶", fmt.Sprintf("queue_next_%d", page))
		if page >= totalPages-1 {
			nextButton = nextButton.AsDisabled()
		}
		components = append(components, discord.NewActionRow(prevButton, nextButton))
	}

	return embed, components
}
