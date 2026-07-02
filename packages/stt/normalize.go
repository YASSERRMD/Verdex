package stt

import "fmt"

const (
	// DefaultSampleRateHz is applied to an AudioInput whose SampleRateHz is
	// unset (zero) when Normalize is called.
	DefaultSampleRateHz = 16000

	// DefaultChannels is applied to an AudioInput whose Channels is unset
	// (zero) when Normalize is called.
	DefaultChannels = 1
)

// Normalize returns a copy of input with sample-rate and channel metadata
// filled in from defaults where unset (zero). It does not touch input.Data
// or DurationMS; no real audio decoding/resampling occurs here — this
// package operates purely on declared metadata, deferring actual codec work
// to concrete provider adapters.
//
// Normalize returns ErrEmptyAudio if input.Data is empty.
func Normalize(input AudioInput) (AudioInput, error) {
	if len(input.Data) == 0 {
		return AudioInput{}, ErrEmptyAudio
	}

	out := input
	if out.SampleRateHz == 0 {
		out.SampleRateHz = DefaultSampleRateHz
	}
	if out.Channels == 0 {
		out.Channels = DefaultChannels
	}
	return out, nil
}

// Chunk is one bounded slice of an AudioInput produced by Segment.
type Chunk struct {
	// Data is the byte slice for this chunk (a sub-slice of the original
	// AudioInput.Data; callers must not mutate it in place after
	// segmentation if the original AudioInput is still in use).
	Data []byte

	// OffsetMS is the start offset of this chunk relative to the beginning
	// of the original audio, in milliseconds.
	OffsetMS int64

	// DurationMS is the duration represented by this chunk, in milliseconds.
	DurationMS int64
}

// Segment splits input into a sequence of bounded Chunks, each spanning at
// most maxChunkMS of audio. Segmentation is deterministic: byte and time
// offsets are computed by a fixed proportional split of input.Data against
// input.DurationMS, so the same input always produces the same chunk
// boundaries. No audio codec is involved — bytes are split purely by
// position, which is sufficient for bounding provider request sizes in this
// model-agnostic pipeline.
//
// Segment returns ErrEmptyAudio if input.Data is empty, and a wrapped
// ErrInvalidRequest if maxChunkMS <= 0.
//
// When input.DurationMS is zero (unknown), the entire payload is returned as
// a single chunk with DurationMS 0, since no time-based split is possible.
func Segment(input AudioInput, maxChunkMS int64) ([]Chunk, error) {
	if len(input.Data) == 0 {
		return nil, ErrEmptyAudio
	}
	if maxChunkMS <= 0 {
		return nil, fmt.Errorf("stt: Segment: %w: maxChunkMS must be positive", ErrInvalidRequest)
	}

	if input.DurationMS <= 0 {
		return []Chunk{{
			Data:       input.Data,
			OffsetMS:   0,
			DurationMS: 0,
		}}, nil
	}

	if input.DurationMS <= maxChunkMS {
		return []Chunk{{
			Data:       input.Data,
			OffsetMS:   0,
			DurationMS: input.DurationMS,
		}}, nil
	}

	numChunks := (input.DurationMS + maxChunkMS - 1) / maxChunkMS
	totalBytes := int64(len(input.Data))
	bytesPerMS := float64(totalBytes) / float64(input.DurationMS)

	chunks := make([]Chunk, 0, numChunks)
	var byteOffset int64
	for i := int64(0); i < numChunks; i++ {
		startMS := i * maxChunkMS
		endMS := startMS + maxChunkMS
		if endMS > input.DurationMS {
			endMS = input.DurationMS
		}
		durMS := endMS - startMS

		var endByte int64
		if i == numChunks-1 {
			endByte = totalBytes
		} else {
			endByte = int64(float64(endMS) * bytesPerMS)
			if endByte > totalBytes {
				endByte = totalBytes
			}
			if endByte < byteOffset {
				endByte = byteOffset
			}
		}

		chunks = append(chunks, Chunk{
			Data:       input.Data[byteOffset:endByte],
			OffsetMS:   startMS,
			DurationMS: durMS,
		})
		byteOffset = endByte
	}

	return chunks, nil
}
