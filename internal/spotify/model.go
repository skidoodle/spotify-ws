package spotify

// TrackItem represents the track object from the Spotify API.
type TrackItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	DurationMs int    `json:"duration_ms"`
	Artists    []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
	} `json:"album"`
}

// CurrentlyPlaying represents the currently playing object from the Spotify API.
// The Item field is a pointer to handle cases where nothing is playing (item is null).
type CurrentlyPlaying struct {
	IsPlaying  bool       `json:"is_playing"`
	ProgressMs int        `json:"progress_ms"`
	Timestamp  int64      `json:"timestamp"`
	Item       *TrackItem `json:"item"`
}
