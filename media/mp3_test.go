package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"slices"
	"testing"
	"time"
)

func TestSplitStereoPCM(t *testing.T) {
	for _, tt := range []struct {
		name   string
		endMS  int
		pauses [][2]int
		wantMS []int
	}{
		{"latest pause", 6500, [][2]int{{2200, 2600}, {3200, 3800}}, []int{3500, 3000}},
		{"brief lull is not a pause", 4500, [][2]int{{3400, 3600}}, []int{4000, 500}},
		{"pause outside search", 4500, [][2]int{{1000, 1600}}, []int{4000, 500}},
		{"exact limit needs no split", 4000, [][2]int{{3200, 3800}}, []int{4000}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stereo, wantPCM, gotPCM []byte
			for i := 0; i < tt.endMS*(wavSampleRate/1000); i++ {
				level := uint16(8000)
				for _, p := range tt.pauses {
					if ms := i / (wavSampleRate / 1000); p[0] <= ms && ms < p[1] {
						level = 0
					}
				}
				stereo = binary.LittleEndian.AppendUint16(stereo, level)
				stereo = binary.LittleEndian.AppendUint16(stereo, level/2)
				wantPCM = binary.LittleEndian.AppendUint16(wantPCM, level*3/4)
			}
			var gotMS []int
			err := splitStereoPCM(t.Context(), bytes.NewReader(stereo), wavSampleRate, 4*time.Second, 2*time.Second, func(wav io.Reader) error {
				data, err := io.ReadAll(wav)
				if err != nil {
					return err
				}
				checkWAV(t, data)
				gotMS = append(gotMS, (len(data)-44)/(2*wavSampleRate/1000))
				gotPCM = append(gotPCM, data[44:]...)
				return nil
			})
			if err != nil || !slices.Equal(gotMS, tt.wantMS) || !bytes.Equal(gotPCM, wantPCM) {
				t.Fatalf("chunks %v ms, want %v ms; PCM bytes %d, want %d; err %v", gotMS, tt.wantMS, len(gotPCM), len(wantPCM), err)
			}
		})
	}
}

func TestSplitStereoPCMResampling(t *testing.T) {
	const rate = 44100
	stereo := bytes.Repeat([]byte{0x00, 0xfe, 0x00, 0xfc}, rate) // left -512, right -1024
	var pcm []byte
	err := splitStereoPCM(context.Background(), bytes.NewReader(stereo), rate, maxChunkDuration, pauseSearch, func(wav io.Reader) error {
		data, err := io.ReadAll(wav)
		checkWAV(t, data)
		pcm = append(pcm, data[44:]...)
		return err
	})
	if err != nil || !bytes.Equal(pcm, bytes.Repeat([]byte{0x00, 0xfd}, wavSampleRate)) {
		t.Fatalf("resampled audio: %d bytes, err %v", len(pcm), err)
	}
}

func checkWAV(t *testing.T, data []byte) {
	t.Helper()
	if len(data) < 44 || string(data[:4]) != "RIFF" || string(data[8:16]) != "WAVEfmt " ||
		binary.LittleEndian.Uint16(data[22:]) != 1 || binary.LittleEndian.Uint32(data[24:]) != wavSampleRate ||
		binary.LittleEndian.Uint16(data[34:]) != 16 || string(data[36:40]) != "data" ||
		int(binary.LittleEndian.Uint32(data[4:])) != len(data)-8 || int(binary.LittleEndian.Uint32(data[40:])) != len(data)-44 {
		t.Fatal("invalid WAV header")
	}
}
