package media

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/hajimehoshi/go-mp3"
)

const (
	wavSampleRate    = 16000
	maxChunkDuration = 5 * time.Minute
	pauseSearch      = 20 * time.Second
	quietWindow      = wavSampleRate / 50 // 20 ms
	minQuietWindows  = 18                 // 360 ms
	quietRMS         = 327                // about -40 dBFS
)

// ErrMP3DecodeStart means the local decoder could not open the stream. A
// caller can still give the original file to a provider that handles it.
var ErrMP3DecodeStart = errors.New("cannot start decoding MP3")

// AudioSplitter emits independently playable chunks in order. Each reader
// must be consumed before the callback returns.
type AudioSplitter interface {
	Split(ctx context.Context, audio io.Reader, emit func(io.Reader) error) error
}

func NewMP3Splitter() AudioSplitter { return mp3Splitter{} }

type mp3Splitter struct{}

func (mp3Splitter) Split(ctx context.Context, audio io.Reader, emit func(io.Reader) error) error {
	stream := bufio.NewReader(contextReader{ctx, audio})
	if err := skipID3v2(stream); err != nil {
		return fmt.Errorf("%w: %w", ErrMP3DecodeStart, err)
	}
	// Do not expose Seek: go-mp3 would scan and index the entire file upfront.
	decoder, err := mp3.NewDecoder(stream)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrMP3DecodeStart, err)
	}
	return splitStereoPCM(ctx, decoder, decoder.SampleRate(), maxChunkDuration, pauseSearch, emit)
}

// Skip leading metadata with bounded memory; the decoder would allocate the
// tag's entire declared size just to discard it.
func skipID3v2(stream *bufio.Reader) error {
	for {
		prefix, err := stream.Peek(3)
		if err != nil || string(prefix) != "ID3" {
			return err
		}
		header, err := stream.Peek(10)
		if err != nil {
			return err
		}
		size := 0
		for _, b := range header[6:10] {
			if b&0x80 != 0 {
				return errors.New("invalid ID3 tag size")
			}
			size = size<<7 | int(b)
		}
		if header[3] == 4 && header[5]&0x10 != 0 {
			size += 10 // ID3v2.4 footer
		}
		if _, err := stream.Discard(10 + size); err != nil {
			return err
		}
	}
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func splitStereoPCM(ctx context.Context, source io.Reader, sourceRate int, limit, search time.Duration, emit func(io.Reader) error) error {
	maxBytes := int(limit*time.Duration(wavSampleRate)/time.Second) * 2
	searchSamples := int(search * time.Duration(wavSampleRate) / time.Second)
	pcm := make([]byte, 0, maxBytes+32*1024)
	var block [16 * 1024]byte
	resampler := monoResampler{rate: sourceRate}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := io.ReadFull(source, block[:])
		end := err == io.EOF || err == io.ErrUnexpectedEOF
		if err != nil && !end {
			return fmt.Errorf("decode MP3: %w", err)
		}
		if n%4 != 0 {
			return errors.New("MP3 decoder returned an incomplete PCM sample")
		}
		pcm = resampler.append(pcm, block[:n])
		if end && resampler.used != 0 {
			pcm = binary.LittleEndian.AppendUint16(pcm, uint16(resampler.sum/int64(resampler.used)))
		}
		for len(pcm) > maxBytes {
			cut := 2 * pauseCut(pcm[:maxBytes], searchSamples)
			if err := emit(wavReader(pcm[:cut])); err != nil {
				return err
			}
			pcm = pcm[:copy(pcm, pcm[cut:])]
		}
		if end {
			if len(pcm) != 0 {
				return emit(wavReader(pcm))
			}
			return nil
		}
	}
}

// Average stereo channels and resample by averaging the input over each
// output sample's interval. Integer weights keep the sample clock exact.
type monoResampler struct {
	rate int
	used int
	sum  int64
}

func (r *monoResampler) append(dst, stereo []byte) []byte {
	for len(stereo) >= 4 {
		left := int32(int16(binary.LittleEndian.Uint16(stereo)))
		right := int32(int16(binary.LittleEndian.Uint16(stereo[2:])))
		stereo = stereo[4:]
		sample := int64(left+right) / 2
		for remaining := wavSampleRate; remaining > 0; {
			weight := min(r.rate-r.used, remaining)
			r.sum += sample * int64(weight)
			r.used += weight
			remaining -= weight
			if r.used == r.rate {
				dst = binary.LittleEndian.AppendUint16(dst, uint16(r.sum/int64(r.rate)))
				r.used, r.sum = 0, 0
			}
		}
	}
	return dst
}

// pauseCut returns the midpoint of the latest sustained quiet interval in
// the search window, or the hard limit if no interval qualifies.
func pauseCut(pcm []byte, searchSamples int) int {
	end := len(pcm) / 2
	start := max(0, end-searchSamples)
	cut, run := end, 0
	for at := start; at+quietWindow <= end; at += quietWindow {
		var energy int64
		for i := at; i < at+quietWindow; i++ {
			s := int64(int16(binary.LittleEndian.Uint16(pcm[2*i:])))
			energy += s * s
		}
		if energy <= int64(quietRMS*quietRMS)*quietWindow {
			run++
			if run >= minQuietWindows {
				cut = at + quietWindow - run*quietWindow/2
			}
		} else {
			run = 0
		}
	}
	return cut
}

func wavReader(pcm []byte) io.Reader {
	header := []byte("RIFF\x00\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x10\x00data\x00\x00\x00\x00")
	binary.LittleEndian.PutUint32(header[4:], uint32(len(header)-8+len(pcm)))
	binary.LittleEndian.PutUint32(header[24:], wavSampleRate)
	binary.LittleEndian.PutUint32(header[28:], wavSampleRate*2)
	binary.LittleEndian.PutUint32(header[40:], uint32(len(pcm)))
	return io.MultiReader(bytes.NewReader(header), bytes.NewReader(pcm))
}
