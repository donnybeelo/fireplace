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
	woodMap     []int       // Stores log ID for each pixel (0 = empty)
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
	
	// Hearth is centered, 90% of width (wider)
	hearthWidth := int(float64(width) * 0.9)
	hearthLeft = (width - hearthWidth) / 2
	hearthRight = hearthLeft + hearthWidth
	
	// Fire simulation grid
	fireHeight = height * 2
	initFire()
	generateLogs()
}

func generateLogs() {
	woodMap = make([]int, width*height)
	
	// Log settings
	logRadius := float64(height) / 8.0
	if logRadius < 2.0 { logRadius = 2.0 }
	
	centerX := float64(hearthLeft + hearthRight) / 2.0
	bottomY := float64(height - 1)
	
	// Define logs (x, y, radius, id)
	type Log struct {
		x, y, r float64
		id      int
	}
	
	logs := []Log{}
	
	// Bottom row (3 logs)
	// Spacing: 2 * radius * overlap
	// X radius is roughly 2.0 * logRadius
	rx := logRadius * 2.2
	
	spacing := rx * 0.9
	
	logs = append(logs, Log{x: centerX, y: bottomY - logRadius*0.8, r: logRadius, id: 1})
	logs = append(logs, Log{x: centerX - spacing, y: bottomY - logRadius*0.6, r: logRadius, id: 2})
	logs = append(logs, Log{x: centerX + spacing, y: bottomY - logRadius*0.6, r: logRadius, id: 3})
	
	// Middle row (2 logs)
	logs = append(logs, Log{x: centerX - spacing*0.5, y: bottomY - logRadius*1.5, r: logRadius*0.9, id: 4})
	logs = append(logs, Log{x: centerX + spacing*0.5, y: bottomY - logRadius*1.5, r: logRadius*0.9, id: 5})
	
	// Top row (1 log)
	logs = append(logs, Log{x: centerX, y: bottomY - logRadius*2.3, r: logRadius*0.8, id: 6})
	
	// Render logs to woodMap
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			for _, l := range logs {
				// Ellipse equation: (dx / rx)^2 + (dy / ry)^2 <= 1
				// rx is width (cols), ry is height (rows)
				// Visually 1 row = 2 cols roughly.
				currentRx := l.r * 2.2
				currentRy := l.r
				
				nx := (float64(x) - l.x) / currentRx
				ny := (float64(y) - l.y) / currentRy
				
				if nx*nx + ny*ny <= 1.0 {
					woodMap[y*width+x] = l.id
				}
			}
		}
	}
}

func initFire() {
	fire = make([]int, width*fireHeight)
}

func updateFire() {
	// Center calculation
	center := (hearthLeft + hearthRight) / 2
	halfWidth := float64(hearthRight - hearthLeft) / 2.0

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
				
				// Distance-based decay to create "peak" in middle
				// normalized distance from center 0..1
				dist := math.Abs(float64(x - center))
				normDist := dist / halfWidth
				
				// Add extra decay based on distance from center
				// This shapes the fire into a cone/peak
				decay += int(normDist * 4.0)
				
				// General height control: shorter fire
				// y=0 is top. If y is small (high up), decay more.
				if y < fireHeight/2 {
					decay += 2
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
	for x := hearthLeft; x < hearthRight; x++ {
		// Log height at this x
		h := getLogHeight(x)
		if h <= 0 { continue }
		
		woodTopDist := h 
		
		// Screen row for top of wood
		screenY := height - 1 - woodTopDist
		
		// Fire grid row
		fireY := screenY * 2
		
		// Add some random depth to make it look like it's coming from inside the pile
		fireY += rand.Intn(6)
		
		if fireY >= fireHeight { fireY = fireHeight - 1 }
		if fireY < 0 { fireY = 0 }

		idx := fireY * width + x
		if idx >= 0 && idx < len(fire) {
			// Randomly vary heat to create flicker and multiple source points
			r := rand.Intn(100)
			heat := 36
			
			// Less uniform ignition
			if r > 40 {
				heat = 0 // Gap
			} else if r > 30 {
				heat = 36 // Hot spot
			} else {
				heat = 20 // Cooler spot
			}
			
			// Always ignite center bottom more
			dist := math.Abs(float64(x - center))
			if dist < 5 && rand.Intn(10) > 2 {
				heat = 36
			}
			
			if heat > 0 {
				fire[idx] = heat
			}
		}
	}
	
	// 3. Embers (Sparks)
	if rand.Intn(100) < 8 { // 8% chance
		rx := hearthLeft + rand.Intn(hearthRight - hearthLeft)
		ry := (height / 2) * 2 + rand.Intn(20)
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
	// Colors for different logs to distinguish them
	woodColors := []tcell.Color{
		tcell.ColorBlack, // 0 placeholder
		tcell.NewRGBColor(139, 69, 19), // SaddleBrown
		tcell.NewRGBColor(160, 82, 45), // Sienna
		tcell.NewRGBColor(205, 133, 63), // Peru
		tcell.NewRGBColor(120, 60, 20),
		tcell.NewRGBColor(150, 75, 40),
		tcell.NewRGBColor(190, 120, 50),
		tcell.NewRGBColor(140, 70, 25),
	}
	
	darkWoodColor := tcell.NewRGBColor(50, 25, 10) 
	ashColor := tcell.NewRGBColor(50, 50, 50)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			logID := 0
			if x >= 0 && x < width && y >= 0 && y < height {
				logID = woodMap[y*width+x]
			}
			
			if logID > 0 {
				// Pick color based on log ID
				colorIdx := (logID - 1) % (len(woodColors) - 1) + 1
				baseColor := woodColors[colorIdx]
				
				// Texture
				noise := (x*57 + y*131) % 10
				var style tcell.Style
				
				if noise > 6 {
					style = tcell.StyleDefault.Background(darkWoodColor).Foreground(baseColor)
				} else {
					style = tcell.StyleDefault.Background(baseColor).Foreground(darkWoodColor)
				}
				
				char := ' '
				if logID % 2 == 0 {
					if (x+y)%4 == 0 { char = '/' }
				} else {
					if (x-y)%4 == 0 { char = '\\' }
				}
				
				// Outline effect for separation
				isEdge := false
				// Check Top (y-1)
				if y > 0 && woodMap[(y-1)*width+x] != logID && woodMap[(y-1)*width+x] != 0 { isEdge = true }
				
				if isEdge {
					style = tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(baseColor)
					char = '_'
				}
				
				screen.SetContent(x, y, char, nil, style)
			} else {
				// Ash
				if y == height - 1 && x >= hearthLeft && x < hearthRight {
					if rand.Intn(10) > 6 {
						screen.SetContent(x, y, '.', nil, tcell.StyleDefault.Foreground(ashColor))
					}
				}
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

// Returns the height of the wood from the bottom at column x
func getLogHeight(x int) int {
	if x < 0 || x >= width {
		return 0
	}
	// Scan from top (0) to bottom (height-1)
	for y := 0; y < height; y++ {
		if woodMap[y*width+x] != 0 {
			// Found top of wood
			return height - 1 - y
		}
	}
	return 0
}

func isWood(x, y int) bool {
	if x < 0 || x >= width || y < 0 || y >= height {
		return false
	}
	return woodMap[y*width+x] != 0
}
