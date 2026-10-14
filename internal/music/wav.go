package music

import (
	"encoding/binary"
	"io"
	"math"
)

// WriteWAV renders one pass of the song as 16-bit stereo PCM, for the
// docs' sample.
func WriteWAV(w io.Writer) error {
	n := Length()
	data := uint32(n * 4)
	hdr := []any{
		[4]byte{'R', 'I', 'F', 'F'}, 36 + data, [4]byte{'W', 'A', 'V', 'E'},
		[4]byte{'f', 'm', 't', ' '}, uint32(16), uint16(1), uint16(2),
		uint32(SampleRate), uint32(SampleRate * 4), uint16(4), uint16(16),
		[4]byte{'d', 'a', 't', 'a'}, data,
	}
	for _, v := range hdr {
		if err := binary.Write(w, binary.LittleEndian, v); err != nil {
			return err
		}
	}
	t := NewTune()
	buf := make([][2]float64, SampleRate)
	pcm := make([]int16, 2*len(buf))
	for n > 0 {
		chunk := buf[:min(n, len(buf))]
		t.Stream(chunk)
		for i, s := range chunk {
			pcm[2*i] = int16(math.Round(s[0] * math.MaxInt16))
			pcm[2*i+1] = int16(math.Round(s[1] * math.MaxInt16))
		}
		if err := binary.Write(w, binary.LittleEndian, pcm[:2*len(chunk)]); err != nil {
			return err
		}
		n -= len(chunk)
	}
	return nil
}
