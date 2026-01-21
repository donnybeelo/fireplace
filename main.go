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

// Doom fire palette definition (RGB) - Cleaned to remove all yellow/white
var palette = []uint32{
	0x070707, 0x1F0707, 0x2F0F07, 0x470F07, 0x571707, 0x671F07, 0x771F07, 0x8F2707,
	0x9F2F07, 0xAF3F07, 0xBF4707, 0xC74707, 0xDF4F07, 0xAF3F07, 0xAF3F07, 0xAF3F07,
	0xAF3F07, 0xAF3F07, 0xAF3F07, 0xAF3F07, 0xAF3F07, 0xAF3F07, 0xAF3F07, 0xAF3F07,
	0xAF3F07, 0xAF3F07, 0xAF3F07, 0xAF3F07, 0xAF3F07, 0xAF3F07, 0xAF3F07, 0xAF3F07,
	0xAF3F07, 0xAF3F07, 0xAF3F07, 0xAF3F07,
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

	screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorBlack))
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
			
			screen.Clear()
			
			// Layered rendering
			drawEnvironment(1, logCount/2)
			drawFire()
			drawEnvironment(logCount/2+1, logCount)
			
			screen.Show()
		}
	}
}

func resize() {
	width, height = screen.Size()
	
	// Hearth fills the entire screen
	hearthLeft = 0
	hearthRight = width
	
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
	baseRX := float64(hearthRight-hearthLeft) * 0.5
	baseRY := float64(height) / 20.0 
	
	baseRadius := float64(height) / 40.0
	if baseRadius < 0.6 { baseRadius = 0.6 }

	type Log struct {
		x1, y1, x2, y2 float64
		r              float64
		depth          float64 
		id             int
	}
	
	logs := []Log{}
	numLogs := 60 + rand.Intn(20) 
	
	for i := 0; i < numLogs; i++ {
		theta := rand.Float64() * 2.0 * math.Pi
		dist := 0.2 + rand.Float64()*0.8
		
		x1 := centerX + baseRX * math.Cos(theta) * dist
		y1 := (bottomY - baseRY) + baseRY * math.Sin(theta) * dist
		
		x2 := peakX + (rand.Float64()-0.5)*10.0
		y2 := peakY + (rand.Float64()-0.5)*5.0
		
		ext := 0.7 + rand.Float64()*0.6
		dx, dy := x2-x1, y2-y1
		x2 = x1 + dx*ext
		y2 = y1 + dy*ext
		
		logs = append(logs, Log{
			x1: x1, y1: y1, x2: x2, y2: y2,
			r: baseRadius * (0.6 + rand.Float64()*0.8),
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
	center := float64(width) / 2.0
	halfWidth := float64(width) / 2.0

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
				// Minimal jitter for propagation only
				drift := rand.Intn(3) - 1
				dstX := x + drift
				if dstX < 0 { dstX = 0 } else if dstX >= width { dstX = width - 1 }

				dstIndex := (y-1)*width + dstX
				if dstIndex < 0 { continue }

				// Stable decay based on normal distribution
				// decay = constant + dist_from_center^2
				dist := math.Abs(float64(x) - center)
				normDist := dist / halfWidth
				
				// Normal distribution-ish falloff
				decay := 1 + int(normDist * normDist * 15.0)
				
				// Standard upward cooling
				if y < fireHeight/2 {
					decay += 1
				}

				newHeat := pixel - decay
				if newHeat < 0 { newHeat = 0 }
				fire[dstIndex] = newHeat
			}
		}
	}

	// 2. Stable Refuel
	for x := 0; x < width; x++ {
		h := getLogHeight(x)
		if h <= 0 { continue }
		
		// Constant heat injection at the base of the logs
		// Biased to center
		dist := math.Abs(float64(x) - center)
		normDist := dist / halfWidth
		
		// Higher probability of heat in the center
		if rand.Float64() > normDist*0.8 {
			// Inject heat at various depths within logs
			for i := 0; i < 2; i++ {
				d := rand.Intn(h + 1)
				fireY := (height - 1 - d) * 2
				if fireY >= 0 && fireY < fireHeight {
					fire[fireY*width+x] = 36
				}
			}
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

			// Get existing background (logs or black)
			_, _, existingStyle, _ := screen.GetContent(x, y)
			_, existingBg, _ := existingStyle.Decompose()

			c1 := colors[clamp(heat1)]
			c2 := colors[clamp(heat2)]

			// If a half-block has no fire, use the existing background color
			if heat1 == 0 {
				c1 = existingBg
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
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			logID := 0
			if x >= 0 && x < width && y >= 0 && y < height {
				logID = woodMap[y*width+x]
			}
			
			if logID >= minID && logID <= maxID {
				// Calculate depth factor (0.0 back to 1.0 front)
				depth := float64(logID) / float64(logCount)
				
				// Much darker log colors
				r := int32(20 + depth*40)
				g := int32(10 + depth*20)
				b := int32(5 + depth*10)
				
				baseColor := tcell.NewRGBColor(r, g, b)
				darkColor := tcell.NewRGBColor(r/2, g/2, b/2)
				
				noise := (x*57 + y*131) % 10
				var style tcell.Style
				
				// Simple shading based on noise
				if noise > 7 {
					style = tcell.StyleDefault.Background(darkColor).Foreground(baseColor)
				} else {
					style = tcell.StyleDefault.Background(baseColor).Foreground(darkColor)
				}
				
				char := ' '
				
				// Subtle top edge highlight (glow from fire)
				if y > 0 && woodMap[(y-1)*width+x] != logID {
					highlightR := clampFloat32(r+20, 0, 255)
					highlightG := clampFloat32(g+10, 0, 255)
					highlightB := clampFloat32(b+5, 0, 255)
					style = style.Background(tcell.NewRGBColor(int32(highlightR), int32(highlightG), int32(highlightB)))
				}
				
				screen.SetContent(x, y, char, nil, style)
			}
		}
	}
}

func clampFloat32(v, min, max int32) int32 {
	if v < min { return min }
	if v > max { return max }
	return v
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
