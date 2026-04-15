package discord_helper

import (
	"fmt"
	"strconv"
	"strings"

	"mydiscordbot/domain"
)

const (
	TrackBlue     = 0x3498db
	TracksPerPage = 10
	MusicPrefix   = "music"
)

func BuildListPageComponents(tracks []domain.Track, page, totalPages int) (string, []string) {
	start := page * TracksPerPage
	end := start + TracksPerPage
	if end > len(tracks) {
		end = len(tracks)
	}

	msg := fmt.Sprintf("Music Library - Page %d of %d (Total: %d)\n\n", page+1, totalPages, len(tracks))

	for idx := start; idx < end; idx++ {
		title := tracks[idx].Title()
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		msg += fmt.Sprintf("%d. %s\n", idx+1, title)
	}

	var buttons []string
	if totalPages > 1 {
		if page > 0 {
			buttons = append(buttons, fmt.Sprintf("%s_list_prev_%d", MusicPrefix, page))
		}
		if page < totalPages-1 {
			buttons = append(buttons, fmt.Sprintf("%s_list_next_%d", MusicPrefix, page))
		}
	}

	return msg, buttons
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

func BuildQueuePageComponents(tracks []domain.Track, currentTrack string, page, totalPages int) (string, []string) {
	start := page * TracksPerPage
	end := start + TracksPerPage
	if end > len(tracks) {
		end = len(tracks)
	}

	msg := fmt.Sprintf("Music Queue - Page %d of %d\n\n", page+1, totalPages)

	if currentTrack != "" {
		msg += fmt.Sprintf("Now Playing: %s\n\nUp Next:\n─────────────────────\n", currentTrack)
	}

	for i := start; i < end && i < len(tracks); i++ {
		trackNum := i + 1
		title := tracks[i].Title()
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		msg += fmt.Sprintf("%d. %s\n", trackNum, title)
	}

	var buttons []string
	if totalPages > 1 {
		if page > 0 {
			buttons = append(buttons, fmt.Sprintf("queue_prev_%d", page))
		}
		if page < totalPages-1 {
			buttons = append(buttons, fmt.Sprintf("queue_next_%d", page))
		}
	}

	return msg, buttons
}
