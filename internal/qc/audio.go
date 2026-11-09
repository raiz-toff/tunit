package qc

import (
	"encoding/binary"
	"math"
	"os"
)

// generateTestWAV creates a stereo WAV that plays:
//   Left only (2s) → silence → Right only (2s) → silence → Both (2s)
func generateTestWAV(path string) error {
	const (
		sampleRate    = 44100
		channels      = 2
		bitsPerSample = 16
		freq          = 440.0
		amplitude     = 0.7 * 32767
	)

	leftDur := 2.0
	gap := 0.3
	rightDur := 2.0
	bothDur := 2.0
	totalDur := leftDur + gap + rightDur + gap + bothDur
	totalSamples := int(totalDur * sampleRate)

	data := make([]int16, totalSamples*channels)
	for i := 0; i < totalSamples; i++ {
		t := float64(i) / sampleRate
		sample := int16(amplitude * math.Sin(2*math.Pi*freq*t))

		switch {
		case t < leftDur:
			data[i*2] = sample
		case t < leftDur+gap:
			// silence
		case t < leftDur+gap+rightDur:
			data[i*2+1] = sample
		case t < leftDur+gap+rightDur+gap:
			// silence
		default:
			data[i*2] = sample
			data[i*2+1] = sample
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	dataSize := uint32(totalSamples * channels * (bitsPerSample / 8))
	byteRate := uint32(sampleRate * channels * bitsPerSample / 8)
	blockAlign := uint16(channels * bitsPerSample / 8)

	// RIFF header
	f.Write([]byte("RIFF"))
	binary.Write(f, binary.LittleEndian, 36+dataSize)
	f.Write([]byte("WAVE"))

	// fmt chunk
	f.Write([]byte("fmt "))
	binary.Write(f, binary.LittleEndian, uint32(16))
	binary.Write(f, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(f, binary.LittleEndian, uint16(channels))
	binary.Write(f, binary.LittleEndian, uint32(sampleRate))
	binary.Write(f, binary.LittleEndian, byteRate)
	binary.Write(f, binary.LittleEndian, blockAlign)
	binary.Write(f, binary.LittleEndian, uint16(bitsPerSample))

	// data chunk
	f.Write([]byte("data"))
	binary.Write(f, binary.LittleEndian, dataSize)
	return binary.Write(f, binary.LittleEndian, data)
}
