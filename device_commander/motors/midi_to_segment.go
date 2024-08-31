package main

import (
	"fmt"
	"os"
	"sort"

	"gitlab.com/gomidi/midi/v2/smf"
)

func readMIDIFile(filePath string) (*smf.SMF, error) {
	smfFile, err := smf.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading MIDI file: %v", err)
	}
	return smfFile, nil
}

type Segment struct {
	Velocity float64
	Duration int64
}

var selectedTracks = []int{9, 12, 3, 13, 14, 7, 15}

func printMIDIFileInfo(smfFile *smf.SMF, filePath string) {
	fmt.Printf("MIDI file: %s\n", filePath)
	fmt.Printf("Number of tracks: %d\n", len(smfFile.Tracks))

	type trackInfo struct {
		index        int
		eventCount   int
		totalDelta   int64
		zeroDelta    int64
		nonZeroDelta int64
	}

	tracks := make([]trackInfo, len(smfFile.Tracks))
	for i, track := range smfFile.Tracks {
		tracks[i] = trackInfo{index: i + 1, eventCount: len(track)}
		fmt.Printf("Track %d events:\n", i+1)
		for channel := 0; channel < 16; channel++ {
			var hasEvents bool
			var channelEvents []string
			for _, event := range track {
				var ch, vel uint8
				if event.Message.GetNoteOn(&ch, nil, &vel) && ch == uint8(channel) {
					hasEvents = true
					channelEvents = append(channelEvents, fmt.Sprintf("    delta %d - velocity %d", event.Delta, vel))
					tracks[i].totalDelta += int64(event.Delta)
					if vel == 0 {
						tracks[i].zeroDelta += int64(event.Delta)
					} else {
						tracks[i].nonZeroDelta += int64(event.Delta)
					}
				}
			}
			if hasEvents {
				var minVelocity, maxVelocity uint8 = 127, 0
				for _, event := range track {
					var ch, vel uint8
					if event.Message.GetNoteOn(&ch, nil, &vel) && ch == uint8(channel) {
						if vel < minVelocity {
							minVelocity = vel
						}
						if vel > maxVelocity {
							maxVelocity = vel
						}
					}
				}
				fmt.Printf("  Channel %d (Min velocity: %d, Max velocity: %d):\n", channel, minVelocity, maxVelocity)
				for _, eventStr := range channelEvents {
					fmt.Println(eventStr)
				}
			}
		}
	}

	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].eventCount > tracks[j].eventCount
	})

	fmt.Println("\nTracks sorted by number of events:")
	for _, t := range tracks {
		fmt.Printf("  Track %d: %d events\n", t.index, t.eventCount)
	}

	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].totalDelta > tracks[j].totalDelta
	})

	fmt.Println("\nTracks sorted by length of deltas:")
	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].nonZeroDelta > tracks[j].nonZeroDelta
	})

	fmt.Println("\nTracks sorted by non-zero velocity delta:")
	for _, t := range tracks {
		fmt.Printf("  Track %d: Total delta %d (Zero velocity: %d, Non-zero velocity: %d)\n",
			t.index, t.totalDelta, t.zeroDelta, t.nonZeroDelta)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please provide a MIDI file path as an argument")
		os.Exit(1)
	}

	midiFilePath := os.Args[1]

	// Read the MIDI file
	smfFile, err := readMIDIFile(midiFilePath)
	if err != nil {
		fmt.Printf("Error reading MIDI file: %v\n", err)
		os.Exit(1)
	}

	// Print information about the MIDI file and its tracks
	printMIDIFileInfo(smfFile, midiFilePath)
}
