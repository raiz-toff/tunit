package qc

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

func getTermSize() (int, int) {
	rows, cols := 24, 80
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	if out, err := cmd.Output(); err == nil {
		fmt.Sscanf(strings.TrimSpace(string(out)), "%d %d", &rows, &cols)
	}
	return rows, cols
}

func fullScreenColorTest() {
	rows, cols := getTermSize()

	fmt.Print("\033[?1049h\033[?25l") // alt screen, hide cursor
	defer fmt.Print("\033[?1049l\033[?25h\033[0m")

	colors := []struct {
		name    string
		r, g, b int
	}{
		{"WHITE", 255, 255, 255},
		{"BLACK", 0, 0, 0},
		{"RED", 255, 0, 0},
		{"GREEN", 0, 255, 0},
		{"BLUE", 0, 0, 255},
	}

	for i, c := range colors {
		bg := fmt.Sprintf("\033[48;2;%d;%d;%dm", c.r, c.g, c.b)

		var sb strings.Builder
		sb.WriteString("\033[2J\033[H")
		line := bg + strings.Repeat(" ", cols)
		for r := 0; r < rows; r++ {
			sb.WriteString(line)
			if r < rows-1 {
				sb.WriteByte('\n')
			}
		}
		os.Stdout.WriteString(sb.String())

		// Let the tester see pure color first
		time.Sleep(1 * time.Second)

		// Then show instruction
		fg := "\033[38;2;0;0;0m"
		if c.r+c.g+c.b < 200 {
			fg = "\033[38;2;255;255;255m"
		}
		msg := fmt.Sprintf(" %s (%d/%d)  —  Look for dead or stuck pixels  —  Press ENTER ", c.name, i+1, len(colors))
		col := (cols - len(msg)) / 2
		if col < 1 {
			col = 1
		}
		fmt.Printf("\033[%d;%dH%s%s%s\033[0m", rows, col, fg, bg, msg)
		fmt.Scanln()
	}
}

// terminalCameraPreview renders a live camera feed in the terminal using
// truecolor ANSI and half-block characters. captureArgs are the ffmpeg
// input flags (e.g. ["-f","v4l2","-i","/dev/video0"]).
func terminalCameraPreview(captureArgs []string, dur time.Duration) error {
	rows, cols := getTermSize()

	camRows := rows - 3
	if camRows < 8 {
		camRows = 8
	}
	camCols := cols
	if camCols > 120 {
		camCols = 120
	}
	imgW := camCols
	imgH := camRows * 2 // half-block doubles vertical resolution

	args := []string{"-nostdin", "-loglevel", "quiet"}
	args = append(args, captureArgs...)
	args = append(args,
		"-vf", fmt.Sprintf("scale=%d:%d", imgW, imgH),
		"-r", "12",
		"-pix_fmt", "rgb24",
		"-f", "rawvideo",
		"pipe:1",
	)

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = nil
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}

	go func() {
		time.Sleep(dur)
		cmd.Process.Kill()
	}()

	fmt.Print("\033[?25l") // hide cursor
	defer fmt.Print("\033[?25h\033[0m\n")

	frameSize := imgW * imgH * 3
	buf := make([]byte, frameSize)

	for {
		if _, err := io.ReadFull(stdout, buf); err != nil {
			break
		}
		renderFrame(buf, imgW, imgH, camRows)
	}

	cmd.Wait()
	return nil
}

func renderFrame(pixels []byte, w, h, displayRows int) {
	var sb strings.Builder
	sb.Grow(w * displayRows * 40)
	sb.WriteString("\033[H")

	for y := 0; y < h; y += 2 {
		for x := 0; x < w; x++ {
			ti := (y*w + x) * 3
			tR, tG, tB := pixels[ti], pixels[ti+1], pixels[ti+2]

			var bR, bG, bB byte
			if y+1 < h {
				bi := ((y+1)*w + x) * 3
				bR, bG, bB = pixels[bi], pixels[bi+1], pixels[bi+2]
			}

			fmt.Fprintf(&sb, "\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm▀",
				tR, tG, tB, bR, bG, bB)
		}
		sb.WriteString("\033[0m\n")
	}

	sb.WriteString("  \033[1m📷 Live camera preview\033[0m")
	os.Stdout.WriteString(sb.String())
}
