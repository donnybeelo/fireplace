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
	logCount    int         // Number of logs generated
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
			
			// Layered rendering
			screen.Clear()
			
			// 1. Draw logs in the back half
			drawEnvironment(1, logCount/2)
			
			// 2. Draw fire (merges with back logs)
			drawFire()
			
			// 3. Draw logs in the front half (on top of fire)
			drawEnvironment(logCount/2+1, logCount)
			
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
	peakY := bottomY - float64(height)/12.0
	
	// Base dimensions
	baseRX := float64(hearthRight-hearthLeft) * 0.45
	baseRY := float64(height) / 18.0
	
	// Thicker sticks for a less "goofy" look
	baseRadius := float64(height) / 22.0 
	if baseRadius < 1.2 { baseRadius = 1.2 }

	type Log struct {
		x1, y1, x2, y2 float64
		r              float64
		depth          float64
		id             int
	}
	
	logs := []Log{}
	numLogs := 25 + rand.Intn(10) // Fewer but chunkier sticks
	
	for i := 0; i < numLogs; i++ {
		theta := rand.Float64() * 2.0 * math.Pi
		dist := 0.1 + rand.Float64()*0.9
		
		x1 := centerX + baseRX * math.Cos(theta) * dist
		y1 := (bottomY - baseRY) + baseRY * math.Sin(theta) * dist
		
		// Direct towards center peak with variance
		x2 := peakX + (rand.Float64()-0.5)*15.0
		y2 := peakY + (rand.Float64()-0.5)*8.0
		
		ext := 0.6 + rand.Float64()*0.6
		dx, dy := x2-x1, y2-y1
		x2 = x1 + dx*ext
		y2 = y1 + dy*ext
		
		logs = append(logs, Log{
			x1: x1, y1: y1, x2: x2, y2: y2,
			r: baseRadius * (0.7 + rand.Float64()*0.6),
			depth: y1,
			id: i + 1,
		})
	}
	
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].depth < logs[j].depth
	})
	
	logCount = len(logs)
	for i := range logs {
		logs[i].id = i + 1
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
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
	halfWidth := float64(hearthRight - hearthLeft) / 2.0
	t := float64(tick)

	// Use more heat sources for a smoother, less "finned" look
	numPeaks := 12
	peakPos := make([]float64, numPeaks)
	peakInt := make([]float64, numPeaks)
	for i := 0; i < numPeaks; i++ {
		f := float64(i) / float64(numPeaks-1)
		peakPos[i] = float64(hearthLeft) + f*float64(hearthRight-hearthLeft)
		
		// Each source flickers at its own rate and phase
		freq := 0.05 + math.Sin(float64(i)*1.3)*0.02
		intensity := 0.6 + 0.4*math.Sin(t*freq + float64(i)*2.4)
		intensity *= 0.8 + rand.Float64()*0.4
		
		peakInt[i] = clampFloat(intensity, 0.2, 1.0)
	}

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
				// Independent jitter for each column
				drift := rand.Intn(3) - 1
				dstX := x + drift
				if dstX < 0 { dstX = 0 } else if dstX >= width { dstX = width - 1 }

				dstIndex := (y-1)*width + dstX
				if dstIndex < 0 { continue }

				decay := rand.Intn(2)
				
				// Multi-peak blending
				minNormDist := 10.0
				for i := 0; i < numPeaks; i++ {
					// Blend distance with intensity. Higher intensity = taller flame.
					d := math.Abs(float64(x)-peakPos[i]) / (halfWidth * 0.2 * peakInt[i])
					if d < minNormDist { minNormDist = d }
				}
				// Sharper falloff outside the peak zones
				decay += int(minNormDist * minNormDist * 18.0)
				
				// Height-based decay
				if y < fireHeight/3 {
					heightDecay := 1.0 - (float64(y) / (float64(fireHeight) / 3.0))
					decay += int(heightDecay * 6.0)
				}

				newHeat := pixel - decay
				if newHeat < 0 { newHeat = 0 }
				fire[dstIndex] = newHeat
			}
		}
	}

	// 2. Refuel from WITHIN the bundle
	for x := hearthLeft; x < hearthRight; x++ {
		h := getLogHeight(x)
		if h <= 0 { continue }
		
		// Find intensity influence for fueling
		maxLocalInt := 0.0
		for i := 0; i < numPeaks; i++ {
			dist := math.Abs(float64(x)-peakPos[i]) / (halfWidth * 0.25)
			if dist < 1.0 {
				local := peakInt[i] * (1.0 - dist)
				if local > maxLocalInt { maxLocalInt = local }
			}
		}

		for i := 0; i < 2; i++ {
			d := rand.Intn(h + 1)
			screenY := height - 1 - d
			fireY := screenY * 2 + rand.Intn(2)
			
			if fireY >= fireHeight { fireY = fireHeight - 1 }
			if fireY < 0 { fireY = 0 }

			idx := fireY * width + x
			if idx >= 0 && idx < len(fire) {
				r := rand.Intn(100)
				threshold := 10.0 + (1.0 - maxLocalInt)*80.0
				if float64(r) > threshold {
					fire[idx] = 36
				}
			}
		}
	}
	
	// 3. Embers
	if rand.Intn(100) < 10 {
		rx := hearthLeft + rand.Intn(hearthRight - hearthLeft)
		ry := (height / 2) * 2 + rand.Intn(20)
		idx := ry * width + rx
		if idx >= 0 && idx < len(fire) {
			fire[idx] = 36
		}
	}
}

func clampFloat(v, min, max float64) float64 {
	if v < min { return min }
	if v > max { return max }
	return v
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

			if heat1 == 0 && heat2 == 0 {
				continue
			}

			// Get existing content (the back logs) to merge with fire
			_, _, existingStyle, _ := screen.GetContent(x, y)
			existingFg, existingBg, _ := existingStyle.Decompose()

			c1 := colors[clamp(heat1)]
			c2 := colors[clamp(heat2)]

			// If a half-block has no fire, keep the existing log color
			// Note: logs are drawn using both Fg and Bg depending on texture.
			// To simplify, we'll use the background color as the "log color".
			if heat1 == 0 {
				c1 = existingFg // Often logs use foreground for highlights
				if existingFg == tcell.ColorWhite || existingFg == tcell.ColorBlack {
					c1 = existingBg
				}
			}
			if heat2 == 0 {
				c2 = existingBg
			}

			// Upper half block: Foreground is top (c1), Background is bottom (c2)
			style := tcell.StyleDefault.Foreground(c1).Background(c2)
			screen.SetContent(x, y, '▀', nil, style)
		}
	}
}

func drawEnvironment(minID, maxID int) {
	woodColor := tcell.NewRGBColor(101, 67, 33) // Deep Brown
	darkWoodColor := tcell.NewRGBColor(60, 30, 10)
	
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			logID := 0
			if x >= 0 && x < width && y >= 0 && y < height {
				logID = woodMap[y*width+x]
			}
			
			// Only draw logs in the specified ID range
			if logID >= minID && logID <= maxID {
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
