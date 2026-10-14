// Command music-sample renders one pass of the Gotham theme to a WAV file,
// for the docs' audio sample (make music-sample).
package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/krank56/jenklod-batman/internal/music"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: music-sample OUT.wav")
		os.Exit(2)
	}
	if err := render(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "music-sample:", err)
		os.Exit(1)
	}
}

func render(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	if err := music.WriteWAV(w); err != nil {
		f.Close()
		return err
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
