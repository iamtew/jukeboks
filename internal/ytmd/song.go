package ytmd

import (
	"fmt"
	"strings"
)

type responseEnvelope struct {
	ExitCode int    `json:"exitCode"`
	Message  string `json:"message,omitempty"`
	Data     any    `json:"data,omitempty"`
}

type CurrentSong struct {
	Title  string
	Artist string
	State  string
}

func ParseCurrentSong(songPayload any) CurrentSong {
	songData := map[string]any{}
	if songPayloadMap, ok := songPayload.(map[string]any); ok {
		songData = songPayloadMap
	}
	if nested, ok := songPayloadMapValue(songData, "data"); ok {
		for key, value := range nested {
			songData[key] = value
		}
	}
	if nested, ok := songPayloadMapValue(songData, "song"); ok {
		for key, value := range nested {
			songData[key] = value
		}
	}

	title := asString(songData["title"])
	artist := asString(songData["artist"])
	if title == "" {
		title = asString(songData["name"])
	}
	if artist == "" {
		artist = asString(songData["artistName"])
	}

	isPaused := false
	if paused, ok := songData["isPaused"].(bool); ok {
		isPaused = paused
	}
	if !isPaused {
		if playing, ok := songData["isPlaying"].(bool); ok {
			isPaused = !playing
		}
	}
	if !isPaused {
		if state, ok := songData["state"].(string); ok {
			isPaused = strings.EqualFold(state, "paused")
		}
	}

	state := "playing"
	if isPaused {
		state = "paused"
	}
	if title == "" && artist == "" {
		state = ""
	}

	return CurrentSong{Title: title, Artist: artist, State: state}
}

func buildSongInfoResponse(songPayload any) responseEnvelope {
	song := ParseCurrentSong(songPayload)
	title := song.Title
	artist := song.Artist
	state := song.State

	if title == "" && artist == "" {
		return responseEnvelope{ExitCode: 1, Message: "Song unavailable. No current track information was returned by the player.", Data: map[string]any{"reason": "no_song"}}
	}

	message := fmt.Sprintf("Song: %s — %s. Playback state: %s.", title, artist, state)
	return responseEnvelope{ExitCode: 0, Message: message, Data: map[string]any{"title": title, "artist": artist, "state": state}}
}

func BuildSongInfoResponse(songPayload any) any {
	return buildSongInfoResponse(songPayload)
}
