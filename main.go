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
	
	centerX := float64(hearthLeft + hearthRight) / 2.0
	bottomY := float64(height - 1)
	
	aspect := 2.0
	
	// Settings
	baseRadius := float64(height) / 22.0
	if baseRadius < 1.2 { baseRadius = 1.2 }
	
	// Shorter logs (was / 2.0)
	logLen := float64(hearthRight - hearthLeft) / 3.5
	
	type Log struct {
		x1, y1, x2, y2 float64
		r              float64
		id             int
	}
	
	logs := []Log{}
	
	// 1. Base logs (more of them)
	numBase := 4 + rand.Intn(3)
	for i := 0; i < numBase; i++ {
		r := (rand.Float64() - 0.5)
		// Spread across the hearth
		offset := r * float64(hearthRight-hearthLeft) * 0.7
		cx := centerX + offset
		
		angle := (rand.Float64() - 0.5) * 0.1
		l := logLen * (0.8 + rand.Float64()*0.4)
		
		x1 := cx - (math.Cos(angle) * l / 2.0)
		y1 := bottomY - baseRadius*0.8 - (math.Sin(angle) * l / 2.0)
		x2 := cx + (math.Cos(angle) * l / 2.0)
		y2 := bottomY - baseRadius*0.8 + (math.Sin(angle) * l / 2.0)
		
		logs = append(logs, Log{
			x1: x1, y1: y1, x2: x2, y2: y2,
			r: baseRadius * (0.9 + rand.Float64()*0.2),
			id: i + 1,
		})
	}
	
	// 2. Stacked Layer (Leaning upwards towards center, more logs)
	numStack := 8 + rand.Intn(5)
	for i := 0; i < numStack; i++ {
		r := (rand.Float64() - 0.5)
		offset := r * float64(hearthRight-hearthLeft) * 0.6
		cx := centerX + offset
		
		cy := bottomY - baseRadius * 2.2 - (rand.Float64() * baseRadius * 3.0)
		
		leanAmt := offset / (float64(hearthRight-hearthLeft) * 0.5) 
		angle := leanAmt * 0.4
		angle += (rand.Float64() - 0.5) * 0.2
		
		l := logLen * (0.7 + rand.Float64()*0.5)
		
		x1 := cx - (math.Cos(angle) * l / 2.0)
		y1 := cy - (math.Sin(angle) * l / 2.0)
		x2 := cx + (math.Cos(angle) * l / 2.0)
		y2 := cy + (math.Sin(angle) * l / 2.0)
		
		logs = append(logs, Log{
			x1: x1, y1: y1, x2: x2, y2: y2,
			r: baseRadius * (0.8 + rand.Float64()*0.3),
			id: numBase + i + 1,
		})
	}

	// Render logs
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
				dist := math.Abs(float64(x - center))
				normDist := dist / halfWidth
				
				// Slightly increased decay compared to before to shrink fire
				decay += int(normDist * 3.0)
				
				// General height control
				if y < fireHeight/3 {
					decay += 1
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
		// Log height at this x
		h := getLogHeight(x)
		
		// Fire ONLY comes from logs
		if h <= 0 { continue }
		
		woodTopDist := h 
		// Screen row for top of wood
		screenY := height - 1 - woodTopDist
		
		// Fire grid row
		fireY := screenY * 2
		
		// Add some random depth
		fireY += rand.Intn(6)
		
		if fireY >= fireHeight { fireY = fireHeight - 1 }
		if fireY < 0 { fireY = 0 }

		idx := fireY * width + x
		if idx >= 0 && idx < len(fire) {
			r := rand.Intn(100)
			heat := 36
			
			// Less uniform ignition
			if r > 50 {
				heat = 0 // Gap
			} else if r > 20 {
				heat = 36 // Hot spot
			} else {
				heat = 25 // Cooler spot
			}
			
			// Always ignite center bottom more
			dist := math.Abs(float64(x - center))
			if dist < 6 && rand.Intn(10) > 2 {
				heat = 36
			}
			
			if heat > 0 {
				fire[idx] = heat
			}
		}
	}
	
	// 3. Embers (Sparks)
	if rand.Intn(100) < 10 { // Slightly reduced chance
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
