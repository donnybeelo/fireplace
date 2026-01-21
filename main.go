package main

import (
	"math"
	"math/rand"
	"sort"
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
	tick        int         // Frame counter for animations
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
			tick++
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
	centerX := float64(hearthLeft + hearthRight) / 2.0
	bottomY := float64(height - 1)
	aspect := 2.0

	// The central peak where sticks converge
	peakX := centerX
	peakY := bottomY - float64(height)/10.0
	
	// Base dimensions (ellipse on the "ground")
	baseRX := float64(hearthRight-hearthLeft) * 0.4
	baseRY := float64(height) / 25.0 // Shallow for depth projection
	
	baseRadius := float64(height) / 35.0 // Thinner "sticks"
	if baseRadius < 0.7 { baseRadius = 0.7 }

	type Log struct {
		x1, y1, x2, y2 float64
		r              float64
		depth          float64 // Bottom Y-coord for painter's algorithm
		id             int
	}
	
	logs := []Log{}
	numLogs := 35 + rand.Intn(15) // Many sticks
	
	for i := 0; i < numLogs; i++ {
		// Random angle around the center for the base point
		theta := rand.Float64() * 2.0 * math.Pi
		
		// Distance from center at base (with some randomization)
		dist := 0.4 + rand.Float64()*0.6
		
		// Starting point (Ground)
		x1 := centerX + baseRX * math.Cos(theta) * dist
		y1 := (bottomY - baseRY) + baseRY * math.Sin(theta) * dist
		
		// Peak point (where they meet)
		// Pass through a small jittered volume at the peak
		x2 := peakX + (rand.Float64()-0.5)*6.0
		y2 := peakY + (rand.Float64()-0.5)*3.0
		
		// For a "dropped" look, extend some sticks slightly past the peak
		ext := 0.8 + rand.Float64()*0.5
		dx, dy := x2-x1, y2-y1
		x2 = x1 + dx*ext
		y2 = y1 + dy*ext
		
		logs = append(logs, Log{
			x1: x1, y1: y1, x2: x2, y2: y2,
			r: baseRadius * (0.6 + rand.Float64()*0.8),
			depth: y1, // Larger y1 means closer to viewer
			id: i + 1,
		})
	}
	
	// Sort logs by depth: back-most first, front-most last
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].depth < logs[j].depth
	})
	
	// Re-assign IDs for color variety after sort
	for i := range logs {
		logs[i].id = i + 1
	}

	// Render logs
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Painter's algorithm: find front-most log for this pixel
			// Since logs are sorted back-to-front, the last one that matches is front-most.
			for i := len(logs) - 1; i >= 0; i-- {
				l := logs[i]
				px, py := float64(x), float64(y) * aspect
				ax, ay := l.x1, l.y1 * aspect
				bx, by := l.x2, l.y2 * aspect
				
				abx, aby := bx - ax, by - ay
				apx, apy := px - ax, py - ay
				lenSq := abx*abx + aby*aby
				t := (apx*abx + apy*aby) / lenSq
				if t < 0 { t = 0 } else if t > 1 { t = 1 }
				
				cx, cy := ax + t*abx, ay + t*aby
				dx, dy := px - cx, py - cy
				if dx*dx + dy*dy <= (l.r*aspect)*(l.r*aspect) {
					woodMap[y*width+x] = l.id
					break
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
				// Wave effect: horizontal drift based on time (tick) and height (y)
				// Slowly shifts the fire left/right as it rises
				wave := math.Sin(float64(tick)*0.15 + float64(y)*0.1) * 1.5
				
				// Propagation with center-biased jitter + wave drift
				randIdx := rand.Intn(3) // 0, 1, 2
				dstX := x - randIdx + 1 + int(wave)
				
				if dstX < 0 {
					dstX = 0
				} else if dstX >= width {
					dstX = width - 1
				}

				dstIndex := (y-1)*width + dstX
				if dstIndex < 0 {
					continue
				}

				// Base random decay
				decay := rand.Intn(2)
				
				// EVEN Pointier: Non-linear distance decay
				dist := math.Abs(float64(x - center))
				normDist := dist / halfWidth
				// Increased multiplier to 12.0 for a sharper peak
				decay += int(normDist * normDist * 12.0)
				
				// Height-based decay (y=0 is top)
				if y < fireHeight/2 {
					heightDecay := 1.0 - (float64(y) / (float64(fireHeight) / 2.0))
					decay += int(heightDecay * 4.0)
				}

				newHeat := pixel - decay
				if newHeat < 0 {
					newHeat = 0
				}
				fire[dstIndex] = newHeat
			}
		}
	}

	// 2. Refuel from wood surface ONLY
	for x := hearthLeft; x < hearthRight; x++ {
		h := getLogHeight(x)
		if h <= 0 { continue }
		
		woodTopDist := h 
		screenY := height - 1 - woodTopDist
		fireY := screenY * 2
		fireY += rand.Intn(4)
		
		if fireY >= fireHeight { fireY = fireHeight - 1 }
		if fireY < 0 { fireY = 0 }

		idx := fireY * width + x
		if idx >= 0 && idx < len(fire) {
			dist := math.Abs(float64(x - center))
			normDist := dist / halfWidth
			
			r := rand.Intn(100)
			heat := 0
			
			// Tighter ignition core
			threshold := 20.0 + (normDist * 60.0) 
			if float64(r) > threshold {
				heat = 36
				if normDist > 0.5 && r > 85 {
					heat = 20
				}
			}
			
			if heat > 0 {
				fire[idx] = heat
			}
		}
	}
	
	// 3. Embers (Sparks)
	if rand.Intn(100) < 12 {
		rx := hearthLeft + rand.Intn(hearthRight - hearthLeft)
		ry := (height / 2) * 2 + rand.Intn(15)
		idx := ry * width + rx
		if idx >= 0 && idx < len(fire) {
			fire[idx] = 36
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
	woodColor := tcell.NewRGBColor(101, 67, 33) // Deep Brown
	darkWoodColor := tcell.NewRGBColor(60, 30, 10)
	
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			logID := 0
			if x >= 0 && x < width && y >= 0 && y < height {
				logID = woodMap[y*width+x]
			}
			
			// Draw Wood
			if logID > 0 {
				noise := (x*57 + y*131) % 10
				var style tcell.Style
				if noise > 7 {
					style = tcell.StyleDefault.Background(darkWoodColor).Foreground(woodColor)
				} else {
					style = tcell.StyleDefault.Background(woodColor).Foreground(darkWoodColor)
				}
				
				char := ' '
				// Sparse texture
				if noise == 0 { char = '░' }
				
				// Highlight top edge
				if y > 0 && woodMap[(y-1)*width+x] != logID && woodMap[(y-1)*width+x] != 0 {
				    // Edge between logs
				    style = tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(woodColor)
				    char = '_'
				} else if y > 0 && woodMap[(y-1)*width+x] == 0 {
					// Top edge against air
					style = style.Foreground(tcell.NewRGBColor(139, 69, 19))
					char = '▄'
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
