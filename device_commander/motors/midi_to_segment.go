package motors

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gitlab.com/gomidi/midi/v2/smf"
)

func readMIDIFile(filePath string) (*smf.SMF, smf.TimeFormat, error) {
	smfFile, err := smf.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading MIDI file: %v", err)
	}

	return smfFile, smfFile.TimeFormat, nil
}

// type Segment struct {
// 	Velocity float64
// 	Duration int64
// }

type MotorTrackToPattern struct {
	Track int64
	Motor int64
}

var motorTrackMapping = []MotorTrackToPattern{
	{Track: 9, Motor: 0},
	{Track: 12, Motor: 1},
	{Track: 3, Motor: 2},
	{Track: 13, Motor: 3},
	{Track: 14, Motor: 4},
	{Track: 7, Motor: 5},
	{Track: 15, Motor: 6},
}

func printMIDIFileInfo(smfFile *smf.SMF, timeFormat smf.TimeFormat, filePath string) {
	fmt.Printf("MIDI file: %s\n", filePath)
	fmt.Printf("Number of tracks: %d\n", len(smfFile.Tracks))

	// Assume initial tempo of 120 BPM
	tempo := 120.0

	var ticksPerQuarterNote float64
	if metrical, ok := timeFormat.(smf.MetricTicks); ok {
		ticksPerQuarterNote = float64(metrical.Resolution())
		fmt.Printf("Ticks per quarter note: %v\n", ticksPerQuarterNote)
	} else if timecode, ok := timeFormat.(smf.TimeCode); ok {
		fmt.Printf("SMPTE format: %d FPS, %d subframes\n", timecode.FramesPerSecond, timecode.SubFrames)
		// For SMPTE, we'd need to handle this differently
		fmt.Println("SMPTE time format not supported in this example")
		return
	}

	// Calculate milliseconds per tick
	millisPerTick := 60000.0 / (tempo * ticksPerQuarterNote)

	type trackInfo struct {
		index        int
		eventCount   int
		totalDelta   int64
		zeroDelta    int64
		nonZeroDelta int64
	}

	tracks := make([]trackInfo, len(smfFile.Tracks))
	patterns := make(map[int64][]Segment)
	for _, mapping := range motorTrackMapping {
		track := smfFile.Tracks[mapping.Track-1]
		segments := []Segment{}
		for _, event := range track {
			var ch, vel uint8
			if event.Message.GetNoteOn(&ch, nil, &vel) {
				duration := int64(float64(event.Delta) * millisPerTick)
				velocity := float64(vel)
				if vel == 0 {
					velocity = 0
				} else {
					velocity = math.Max(0, math.Min(10, (float64(vel)/127.0)*10.0))
				}
				if duration <= 10000 {
					segments = append(segments, Segment{
						Duration: int(duration * 10),
						Speed:    velocity,
					})
				}
			}
		}
		patterns[mapping.Motor] = segments
	}

	output := struct {
		Patterns []struct {
			MotorId  int64     `json:"motorId"`
			Segments []Segment `json:"segments"`
		} `json:"patterns"`
	}{}

	for motorId, segments := range patterns {
		output.Patterns = append(output.Patterns, struct {
			MotorId  int64     `json:"motorId"`
			Segments []Segment `json:"segments"`
		}{
			MotorId:  motorId,
			Segments: segments,
		})
	}

	jsonData, err := json.MarshalIndent(output, "", "    ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}

	outputFileName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)) + ".json"
	err = ioutil.WriteFile(outputFileName, jsonData, 0644)
	if err != nil {
		fmt.Printf("Error writing JSON file: %v\n", err)
		return
	}

	fmt.Printf("JSON file created: %s\n", outputFileName)

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

func RunMidi() {
	if len(os.Args) < 2 {
		fmt.Println("Please provide a MIDI file path as an argument")
		os.Exit(1)
	}

	midiFilePath := os.Args[1]

	// Read the MIDI file
	smfFile, timeFormat, err := readMIDIFile(midiFilePath)
	if err != nil {
		fmt.Printf("Error reading MIDI file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Time Format: %v\n", timeFormat)

	// Print information about the MIDI file and its tracks
	printMIDIFileInfo(smfFile, timeFormat, midiFilePath)
}
