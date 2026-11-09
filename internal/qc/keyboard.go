package qc

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

type keyInfo struct {
	label string
	row   int
	col   int
	width int
}

var keyLayout = []keyInfo{
	// Row 0: number row
	{"`", 0, 0, 3}, {"1", 0, 4, 3}, {"2", 0, 8, 3}, {"3", 0, 12, 3},
	{"4", 0, 16, 3}, {"5", 0, 20, 3}, {"6", 0, 24, 3}, {"7", 0, 28, 3},
	{"8", 0, 32, 3}, {"9", 0, 36, 3}, {"0", 0, 40, 3}, {"-", 0, 44, 3},
	{"=", 0, 48, 3}, {"BK", 0, 52, 5},
	// Row 1: QWERTY row
	{"TAB", 1, 0, 5}, {"Q", 1, 6, 3}, {"W", 1, 10, 3}, {"E", 1, 14, 3},
	{"R", 1, 18, 3}, {"T", 1, 22, 3}, {"Y", 1, 26, 3}, {"U", 1, 30, 3},
	{"I", 1, 34, 3}, {"O", 1, 38, 3}, {"P", 1, 42, 3}, {"[", 1, 46, 3},
	{"]", 1, 50, 3}, {"\\", 1, 54, 3},
	// Row 2: home row
	{"A", 2, 1, 3}, {"S", 2, 5, 3}, {"D", 2, 9, 3}, {"F", 2, 13, 3},
	{"G", 2, 17, 3}, {"H", 2, 21, 3}, {"J", 2, 25, 3}, {"K", 2, 29, 3},
	{"L", 2, 33, 3}, {";", 2, 37, 3}, {"'", 2, 41, 3}, {"RET", 2, 45, 5},
	// Row 3: bottom row
	{"Z", 3, 2, 3}, {"X", 3, 6, 3}, {"C", 3, 10, 3}, {"V", 3, 14, 3},
	{"B", 3, 18, 3}, {"N", 3, 22, 3}, {"M", 3, 26, 3}, {",", 3, 30, 3},
	{".", 3, 34, 3}, {"/", 3, 38, 3},
	// Row 4: bottom
	{"SPC", 4, 10, 20},
	// Arrows
	{"UP", 5, 38, 4}, {"LF", 5, 42, 4}, {"DN", 5, 46, 4}, {"RT", 5, 50, 4},
	// Escape
	{"ESC", 5, 0, 5},
}

// byteToKey maps a byte (or first byte of sequence) to a key label.
var byteToKey = map[byte]string{
	'`': "`", '~': "`", '1': "1", '!': "1", '2': "2", '@': "2",
	'3': "3", '#': "3", '4': "4", '$': "4", '5': "5", '%': "5",
	'6': "6", '^': "6", '7': "7", '&': "7", '8': "8", '*': "8",
	'9': "9", '(': "9", '0': "0", ')': "0", '-': "-", '_': "-",
	'=': "=", '+': "=",
	'\t': "TAB",
	'q': "Q", 'Q': "Q", 'w': "W", 'W': "W", 'e': "E", 'E': "E",
	'r': "R", 'R': "R", 't': "T", 'T': "T", 'y': "Y", 'Y': "Y",
	'u': "U", 'U': "U", 'i': "I", 'I': "I", 'o': "O", 'O': "O",
	'p': "P", 'P': "P", '[': "[", '{': "[", ']': "]", '}': "]",
	'\\': "\\", '|': "\\",
	'a': "A", 'A': "A", 's': "S", 'S': "S", 'd': "D", 'D': "D",
	'f': "F", 'F': "F", 'g': "G", 'G': "G", 'h': "H", 'H': "H",
	'j': "J", 'J': "J", 'k': "K", 'K': "K", 'l': "L", 'L': "L",
	';': ";", ':': ";", '\'': "'", '"': "'",
	'\r': "RET", '\n': "RET",
	'z': "Z", 'Z': "Z", 'x': "X", 'X': "X", 'c': "C", 'C': "C",
	'v': "V", 'V': "V", 'b': "B", 'B': "B", 'n': "N", 'N': "N",
	'm': "M", 'M': "M", ',': ",", '<': ",", '.': ".", '>': ".",
	'/': "/", '?': "/",
	' ': "SPC",
	127: "BK", 8: "BK",
}

func runVisualKeyboardTest() int {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Println("  Could not enable raw input")
		return 0
	}
	defer term.Restore(fd, oldState)

	pressed := map[string]bool{}
	total := len(keyLayout)
	lastKey := ""
	escCount := 0

	fmt.Print("\033[?1049h\033[?25l") // alt screen, hide cursor

	renderKeyboard(pressed, total, lastKey, "Press keys to test  |  ESC×3 to finish")

	buf := make([]byte, 8)
	timeout := time.After(120 * time.Second)

	for {
		select {
		case <-timeout:
			fmt.Print("\033[?1049l\033[?25h\033[0m")
			return len(pressed)
		default:
		}

		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			continue
		}

		key := ""
		if n == 1 {
			if buf[0] == 27 {
				escCount++
				key = "ESC"
				if escCount >= 3 {
					fmt.Print("\033[?1049l\033[?25h\033[0m")
					return len(pressed)
				}
			} else {
				escCount = 0
				if k, ok := byteToKey[buf[0]]; ok {
					key = k
				}
				// Ctrl+key combos still count as the letter
				if buf[0] >= 1 && buf[0] <= 26 {
					letter := string(rune('A' + buf[0] - 1))
					key = letter
				}
			}
		} else if n >= 3 && buf[0] == 27 && buf[1] == '[' {
			escCount = 0
			switch buf[2] {
			case 'A':
				key = "UP"
			case 'B':
				key = "DN"
			case 'C':
				key = "RT"
			case 'D':
				key = "LF"
			}
		} else {
			escCount = 0
		}

		if key != "" {
			pressed[key] = true
			lastKey = key
			renderKeyboard(pressed, total, lastKey, "Press keys to test  |  ESC×3 to finish")
		}
	}
}

func renderKeyboard(pressed map[string]bool, total int, lastKey, instruction string) {
	var sb strings.Builder
	sb.WriteString("\033[2J\033[H")

	count := len(pressed)
	pct := 0
	if total > 0 {
		pct = count * 100 / total
	}

	// Header
	sb.WriteString("\033[1m  KEYBOARD TEST\033[0m\n\n")

	barWidth := 40
	filled := barWidth * count / total
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	sb.WriteString(fmt.Sprintf("  Progress: \033[32m%s\033[0m  %d/%d keys (%d%%)\n\n", bar, count, total, pct))

	// Render keyboard rows
	for row := 0; row <= 5; row++ {
		topLine := strings.Repeat(" ", 60)
		midLine := strings.Repeat(" ", 60)
		botLine := strings.Repeat(" ", 60)

		for _, k := range keyLayout {
			if k.row != row {
				continue
			}
			tested := pressed[k.label]
			flash := k.label == lastKey

			bg := "\033[48;2;60;60;70m"  // dim gray
			fg := "\033[38;2;140;140;150m"
			if tested {
				bg = "\033[48;2;30;140;60m" // green
				fg = "\033[38;2;255;255;255m"
			}
			if flash {
				bg = "\033[48;2;80;200;120m" // bright green flash
				fg = "\033[38;2;0;0;0m"
			}

			top := bg + strings.Repeat("▄", k.width) + "\033[0m"
			padded := k.label
			if len(padded) < k.width {
				pad := k.width - len(padded)
				left := pad / 2
				right := pad - left
				padded = strings.Repeat(" ", left) + padded + strings.Repeat(" ", right)
			}
			mid := bg + fg + padded + "\033[0m"
			bot := bg + strings.Repeat("▀", k.width) + "\033[0m"

			for i := 0; i < k.width && k.col+i < 60; i++ {
				topBytes := []byte(topLine)
				midBytes := []byte(midLine)
				botBytes := []byte(botLine)
				_ = topBytes
				_ = midBytes
				_ = botBytes
			}

			// Position based rendering
			sb.WriteString(fmt.Sprintf("\033[%d;%dH%s", row*3+5, k.col+4, top))
			sb.WriteString(fmt.Sprintf("\033[%d;%dH%s", row*3+6, k.col+4, mid))
			sb.WriteString(fmt.Sprintf("\033[%d;%dH%s", row*3+7, k.col+4, bot))
		}
	}

	// Last key indicator
	sb.WriteString(fmt.Sprintf("\033[24;4H\033[1m  Last: \033[32m%s\033[0m        ", lastKey))
	sb.WriteString(fmt.Sprintf("\033[25;4H  %s", instruction))

	os.Stdout.WriteString(sb.String())
}
