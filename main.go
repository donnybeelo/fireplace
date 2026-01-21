package main

import (
	"math/rand"
	"time"

	"github.com/gdamore/tcell/v2"
)

var (
	width      int
	height     int // Terminal height
	fireHeight int // Simulation height (height * 2 + seed)
	woodHeight = 4 // Height of the wood base in terminal rows
	sourceRow  int // The row in the simulation grid where fire starts
	screen     tcell.Screen
	fire       []int
	colors     []tcell.Color
)

// Doom fire palette definition (RGB)
var palette = []uint32{
	0x070707, 0x1F0707, 0x2F0F07, 0x470F07, 0x571707, 0x671F07, 0x771F07, 0x8F2707,
	0x9F2F07, 0xAF3F07, 0xBF4707, 0xC74707, 0xDF4F07, 0xDF5707, 0xDF5707, 0xD75F07,
	0xD75F07, 0xD7670F, 0xCF6F0F, 0xCF770F, 0xCF7F0F, 0xCF8717, 0xC78717, 0xC78F17,
	0xC7971F, 0xBF9F1F, 0xBF9F1F, 0xBFA727, 0xBFA727, 0xBFAF2F, 0xB7B72F, 0xB7B737,
	0xCFCF6F, 0xDFDF9F, 0xEFEFC7, 0xFFFFFF,
}

func init() {
	colors = make([]tcell.Color, 37) // 0 to 36
	// Fill 0 with black
	colors[0] = tcell.NewRGBColor(0, 0, 0)

	for i, hex := range palette {
		if i+1 >= len(colors) {
			break
		}
		r := int32((hex >> 16) & 0xFF)
		g := int32((hex >> 8) & 0xFF)
		b := int32(hex & 0xFF)
		colors[i+1] = tcell.NewRGBColor(r, g, b)
	}
}

func main() {
	var err error
	screen, err = tcell.NewScreen()
	if err != nil {
		panic(err)
	}

	if err := screen.Init(); err != nil {
		panic(err)
	}
	defer screen.Fini()

	screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite))
	screen.Clear()

	// Initial setup
	rand.Seed(time.Now().UnixNano())
	resize()

	// Event handling
	events := make(chan tcell.Event)
	go func() {
		for {
			events <- screen.PollEvent()
		}
	}()

	ticker := time.NewTicker(time.Millisecond * 50) // 20 FPS
	defer ticker.Stop()

	for {
		select {
		case ev := <-events:
			switch ev := ev.(type) {
			case *tcell.EventResize:
				screen.Sync()
				resize()
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
					return
				}
			}
		case <-ticker.C:
			updateFire()
			drawFire()
			drawWood()
			screen.Show()
		}
	}
}

func resize() {
	width, height = screen.Size()
	// We double the height for the simulation grid to use half-block chars (2 pixels per char)
	// Add extra rows for the seed at the bottom
	fireHeight = height*2 + 2
	sourceRow = (height - woodHeight) * 2
	initFire()
}

func initFire() {
	fire = make([]int, width*fireHeight)
	// Set the source row to max heat
	rowStart := sourceRow * width
	for x := 0; x < width; x++ {
		// Ensure we don't go out of bounds
		if rowStart+x < len(fire) {
			fire[rowStart+x] = 36
		}
	}
}

func updateFire() {
	// Propagate fire from sourceRow upwards
	// We iterate over the fire grid up to sourceRow.
	for x := 0; x < width; x++ {
		for y := 1; y <= sourceRow; y++ {
			src := y*width + x
			pixel := fire[src]

			if pixel == 0 {
				fire[src-width] = 0
			} else {
				randIdx := rand.Intn(3) // 0, 1, 2
				dstX := x - randIdx + 1
				if dstX < 0 {
					dstX = 0
				} else if dstX >= width {
					dstX = width - 1
				}

				dstIndex := (y-1)*width + dstX

				// Increase decay to shorten the flames (0 or 1, mean 0.5)
				decay := rand.Intn(2)
				newHeat := pixel - decay
				if newHeat < 0 {
					newHeat = 0
				}
				fire[dstIndex] = newHeat
			}
		}
	}
	
	// Replenish source row
	rowStart := sourceRow * width
	for x := 0; x < width; x++ {
		if rowStart+x < len(fire) {
			fire[rowStart+x] = 36
		}
	}
}

func drawFire() {
	// Map simulation grid to terminal grid.
	// We map 2 vertical simulation pixels to 1 terminal character (half block).
	// Row 0 of term = Rows 0 and 1 of sim.

	for y := 0; y < height-woodHeight; y++ {
		for x := 0; x < width; x++ {
			// Sim coordinates
			sy1 := y * 2
			sy2 := y*2 + 1

			// Safety check
			if sy2*width+x >= len(fire) {
				continue
			}

			heat1 := fire[sy1*width+x]
			heat2 := fire[sy2*width+x] // Bottom half

			c1 := colors[clamp(heat1)]
			c2 := colors[clamp(heat2)]

			// Upper half block: Foreground is top (c1), Background is bottom (c2)
			style := tcell.StyleDefault.Foreground(c1).Background(c2)
			screen.SetContent(x, y, '▀', nil, style)
		}
	}
}

func drawWood() {
	woodColor := tcell.NewRGBColor(139, 69, 19) // SaddleBrown
	darkWoodColor := tcell.NewRGBColor(101, 67, 33) // Darker Brown
	
	for y := height - woodHeight; y < height; y++ {
		for x := 0; x < width; x++ {
			// Simple wood texture pattern (deterministic)
			// Use coordinates to generate a static pattern
			noise := (x*57 + y*131) % 10
			
			var style tcell.Style
			if noise > 7 {
				style = tcell.StyleDefault.Background(darkWoodColor).Foreground(woodColor)
			} else {
				style = tcell.StyleDefault.Background(woodColor).Foreground(darkWoodColor)
			}
			
			// Use some characters to give texture
			char := ' '
			if noise == 0 {
				char = '|'
			} else if noise == 1 {
				char = '#'
			}
			
			screen.SetContent(x, y, char, nil, style)
		}
	}
}

func clamp(h int) int {
	if h < 0 {
		return 0
	}
	if h > 36 {
		return 36
	}
	return h
}
