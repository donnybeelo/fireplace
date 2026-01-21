package main

import (
	"math"
	"math/rand"
	"time"

	"github.com/gdamore/tcell/v2"
)

var (
	width       int
	height      int // Terminal height
	fireHeight  int // Simulation height (height * 2 + seed)
	woodHeight  int // Dynamic wood height
	hearthLeft  int // Left boundary of the fireplace
	hearthRight int // Right boundary of the fireplace
	screen      tcell.Screen
	fire        []int
	colors      []tcell.Color
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
			drawEnvironment()
			screen.Show()
		}
	}
}

func resize() {
	width, height = screen.Size()
	
	// Define fireplace dimensions
	woodHeight = height / 5
	if woodHeight < 3 {
		woodHeight = 3
	}
	
	// Hearth is centered, roughly 60% of width
	hearthWidth := int(float64(width) * 0.6)
	hearthLeft = (width - hearthWidth) / 2
	hearthRight = hearthLeft + hearthWidth
	
	// Fire simulation grid
	fireHeight = height * 2
	initFire()
}

func initFire() {
	fire = make([]int, width*fireHeight)
}

func updateFire() {
	// 1. Propagate and decay
	for x := 0; x < width; x++ {
		for y := 1; y < fireHeight; y++ {
			src := y*width + x
			pixel := fire[src]

			if pixel == 0 {
				if src-width >= 0 {
					fire[src-width] = 0
				}
			} else {
				randIdx := rand.Intn(3) // 0, 1, 2
				dstX := x - randIdx + 1
				if dstX < 0 {
					dstX = 0
				} else if dstX >= width {
					dstX = width - 1
				}

				dstIndex := (y-1)*width + dstX
				if dstIndex < 0 {
					continue
				}

				decay := rand.Intn(2) // 0 or 1
				
				// Extra cooling outside the hearth (containment)
				if x < hearthLeft || x >= hearthRight {
					decay += 3 // Die out very fast on sides
				}
				
				// Extra cooling near top to ensure flame height is realistic
				// y=0 is top.
				if y < fireHeight/2 {
					if rand.Intn(10) > 7 {
						decay++
					}
				}

				newHeat := pixel - decay
				if newHeat < 0 {
					newHeat = 0
				}
				fire[dstIndex] = newHeat
			}
		}
	}

	// 2. Refuel from wood surface
	// Only refuel within the hearth
	for x := hearthLeft; x < hearthRight; x++ {
		// Log height at this x
		h := getLogHeight(x)
		base := woodHeight / 3
		woodTopDist := base + h
		
		// Screen row for top of wood
		screenY := height - 1 - woodTopDist
		
		// Fire grid row (approximate)
		// We want to ignite the pixels just *above* or *on* the wood.
		// Since y=0 is top, larger y is lower.
		// ignite at the wood surface.
		fireY := screenY * 2
		
		// Add some variability to source depth
		fireY += rand.Intn(3)
		
		if fireY >= fireHeight { fireY = fireHeight - 1 }
		if fireY < 0 { fireY = 0 }

		idx := fireY * width + x
		if idx >= 0 && idx < len(fire) {
			// Randomly vary heat to create flicker
			r := rand.Intn(100)
			heat := 36
			if r > 80 {
				heat = 32 // cooler
			} else if r > 95 {
				heat = 0 // gap
			}
			fire[idx] = heat
		}
	}
	
	// 3. Embers (Sparks)
	// Randomly ignite pixels above the fire source to simulate flying sparks
	if rand.Intn(100) < 15 { // 15% chance
		rx := hearthLeft + rand.Intn(hearthRight - hearthLeft)
		// Spawn in the middle-ish height
		ry := (height / 2) * 2 + rand.Intn(10)
		idx := ry * width + rx
		if idx >= 0 && idx < len(fire) {
			fire[idx] = 36 // Hot spark
		}
	}
}

func drawFire() {
	// Map simulation grid to terminal grid.
	for y := 0; y < height; y++ {
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

func drawEnvironment() {
	woodColor := tcell.NewRGBColor(139, 69, 19) // SaddleBrown
	darkWoodColor := tcell.NewRGBColor(101, 67, 33) // Darker Brown
	brickColor := tcell.NewRGBColor(178, 34, 34) // Firebrick
	mortarColor := tcell.NewRGBColor(100, 100, 100) // Dark Gray for mortar

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 1. Draw Walls (Bricks)
			if x < hearthLeft || x >= hearthRight {
				// Simple brick pattern
				isMortar := false
				if y % 4 == 3 { // Horizontal mortar line every 4th line
					isMortar = true
				} else {
					// Vertical mortar, staggered
					rowBlock := y / 4
					offset := 0
					if rowBlock % 2 == 1 {
						offset = 2
					}
					if (x + offset) % 8 == 0 { // Wider bricks
						isMortar = true
					}
				}
				
				style := tcell.StyleDefault.Background(brickColor).Foreground(mortarColor)
				char := ' '
				if isMortar {
					style = tcell.StyleDefault.Background(mortarColor)
				}
				screen.SetContent(x, y, char, nil, style)
				continue
			}

			// 2. Draw Wood Logs
			if isWood(x, y) {
				// Deterministic noise for texture
				noise := (x*57 + y*131) % 10
				var style tcell.Style
				if noise > 6 {
					style = tcell.StyleDefault.Background(darkWoodColor).Foreground(woodColor)
				} else {
					style = tcell.StyleDefault.Background(woodColor).Foreground(darkWoodColor)
				}
				
				char := ' '
				if noise == 0 {
					char = '|'
				} else if noise == 1 {
					char = '#'
				} else if noise == 2 {
					char = '='
				}
				screen.SetContent(x, y, char, nil, style)
			}
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

// Returns the height of the log "hump" at column x (0 if no log)
func getLogHeight(x int) int {
	if x < hearthLeft || x >= hearthRight {
		return 0
	}
	
	// Normalize x within the hearth
	relX := float64(x - hearthLeft)
	w := float64(hearthRight - hearthLeft)
	
	// 3 distinct humps (logs)
	// sin(x) goes 0->1->0->-1->0
	// We want |sin| or max(0, sin)
	// Let's use absolute sine for a packed look
	val := math.Abs(math.Sin(relX / w * 3 * math.Pi))
	
	// Scale to woodHeight (leaving some space for the base)
	// 2/3 of woodHeight is the hump, 1/3 is the base
	humpH := float64(woodHeight) * 0.7
	
	return int(val * humpH)
}

func isWood(x, y int) bool {
	if x < hearthLeft || x >= hearthRight {
		return false
	}
	// Distance from bottom of screen
	distFromBottom := height - 1 - y
	if distFromBottom < 0 {
		return false
	}
	
	// Base height (always present in hearth)
	base := woodHeight / 3
	
	// Extra height from the log shape
	h := getLogHeight(x)
	
	return distFromBottom < (base + h)
}
