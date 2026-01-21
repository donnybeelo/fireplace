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
	woodMap     []int // Stores log ID for each pixel (0 = empty)
	colors      []tcell.Color
	tick        int // Frame counter for animations
	logCount    int // Number of logs generated
)

// Doom fire palette definition (RGB) - No white/yellow
var palette = []uint32{
	0x070707, 0x1F0707, 0x2F0F07, 0x470F07, 0x571707, 0x671F07, 0x771F07, 0x8F2707,
	0x9F2F07, 0xAF3F07, 0xBF4707, 0xC74707, 0xDF4F07, 0xDF5707, 0xDF5707, 0xD75F07,
	0xD75F07, 0xD7670F, 0xCF6F0F, 0xCF770F, 0xCF7F0F, 0xCF8717, 0xC78717, 0xC78F17,
	0xC7971F, 0xBF9F1F, 0xBF9F1F, 0xBFA727, 0xBFA727, 0xBFAF2F, 0xB7B72F, 0xB7B737,
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

			screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorBlack))
			screen.Clear()

			// 1. Draw all sticks first to establish the woodMap on the screen
			drawEnvironment(1, logCount)

			// 2. Draw fire with blending logic
			drawFireBlended()

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
	centerX := float64(hearthLeft+hearthRight) / 2.0
	bottomY := float64(height - 1)
	aspect := 2.0

	// Sticks should be thin
	baseRadius := float64(height) / 90.0
	if baseRadius < 0.4 {
		baseRadius = 0.4
	}

	type Log struct {
		midX, midY     float64
		dx, dy         float64
		angle          float64
		length         float64
		r              float64
		depth          float64
		id             int
		x1, y1, x2, y2 float64
	}

	tempLogs := []Log{}

	numLogs := 100

	// Ensure we have an even number for pairing

	if numLogs%2 != 0 {

		numLogs++

	}

	sigmaX := float64(width) * 0.25

	// 1. Generate sticks in pairs to ensure balance

	for i := 0; i < numLogs; i += 2 {

		// Sample a distance from center

		offset := math.Abs(rand.NormFloat64() * sigmaX)

		// Attempt to place a pair (left and right)

		for side := 0; side < 2; side++ {

			var midX, midY float64

			var length, angle, r float64

			dir := 1.0

			if side == 0 {

				dir = -1.0

			}

			maxAttempts := 15

			for attempt := 0; attempt < maxAttempts; attempt++ {

				// Each side gets its own variation but same horizontal distance magnitude

				thisOffset := offset * (0.9 + rand.Float64()*0.2)

				midX = centerX + (dir * thisOffset)

				distFromCenter := (midX - centerX) / sigmaX

				maxH := (float64(height) / 3.0) * math.Exp(-distFromCenter*distFromCenter*0.8)

				length = 7.0 + rand.Float64()*12.0

				angle = (rand.Float64() - 0.5) * math.Pi * 0.6

				r = baseRadius * (0.6 + rand.Float64()*0.8)

				limitY := bottomY - r - 0.5

				hRange := maxH

				if hRange > limitY {

					hRange = limitY

				}

				midY = limitY - rand.Float64()*hRange

				if len(tempLogs) < 4 {

					// Seed the first few sticks near the center

					if math.Abs(midX-centerX) < 5.0 {

						break

					}

					continue

				}

				// Proximity check

				isNear := false

				proximityLimit := length * 1.5

				for _, existing := range tempLogs {

					dx := midX - existing.midX

					dy := midY - existing.midY

					if dx*dx+dy*dy < proximityLimit*proximityLimit {

						isNear = true

						break

					}

				}

				if isNear || attempt == maxAttempts-1 {

					break

				}

			}

			tempLogs = append(tempLogs, Log{

				midX: midX, midY: midY,

				angle: angle, length: length, r: r,

				depth: midY, id: len(tempLogs) + 1,
			})

		}

	}

	// 2. Adjust angles: if nothing is underneath the center, make it horizontal
	for i := range tempLogs {
		underneath := false
		for j := range tempLogs {
			if i == j {
				continue
			}
			// Check if log j is "under" log i (larger Y, similar X)
			// Using a small horizontal window to define "under"
			if tempLogs[j].midY > tempLogs[i].midY+0.5 &&
				math.Abs(tempLogs[j].midX-tempLogs[i].midX) < tempLogs[i].length/3.0 {
				underneath = true
				break
			}
		}

		if !underneath {
			tempLogs[i].angle = 0
			// If it's the bottom stick, make sure it's actually near the bottom
			// to look like it's resting on the floor.
			if tempLogs[i].midY > bottomY-5.0 {
				tempLogs[i].midY = bottomY - tempLogs[i].r - 0.2
			}
		}

		// Recalculate x1, y1, x2, y2 based on final angle
		dx := math.Cos(tempLogs[i].angle) * tempLogs[i].length / 2.0
		dy := math.Sin(tempLogs[i].angle) * tempLogs[i].length / 2.0 / aspect

		// Horizontal clamping
		mx := tempLogs[i].midX
		r := tempLogs[i].r
		if mx-math.Abs(dx)-r < 0 {
			mx = math.Abs(dx) + r
		}
		if mx+math.Abs(dx)+r > float64(width-1) {
			mx = float64(width-1) - math.Abs(dx) - r
		}

		tempLogs[i].x1 = mx - dx
		tempLogs[i].y1 = tempLogs[i].midY - dy
		tempLogs[i].x2 = mx + dx
		tempLogs[i].y2 = tempLogs[i].midY + dy
	}

	// Sort logs by depth
	sort.Slice(tempLogs, func(i, j int) bool {
		return tempLogs[i].depth < tempLogs[j].depth
	})

	logCount = len(tempLogs)
	for i := range tempLogs {
		tempLogs[i].id = i + 1
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			for i := len(tempLogs) - 1; i >= 0; i-- {
				l := tempLogs[i]
				px, py := float64(x), float64(y)*aspect
				ax, ay := l.x1, l.y1*aspect
				bx, by := l.x2, l.y2*aspect

				abx, aby := bx-ax, by-ay
				apx, apy := px-ax, py-ay
				lenSq := abx*abx + aby*aby
				if lenSq == 0 {
					continue
				}
				t := (apx*abx + apy*aby) / lenSq
				if t < 0 {
					t = 0
				} else if t > 1 {
					t = 1
				}

				cx, cy := ax+t*abx, ay+t*aby
				dx, dy := px-cx, py-cy
				if dx*dx+dy*dy <= (l.r*aspect)*(l.r*aspect) {
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

	// Clear the top row of fire to prevent "hanging" artifacts
	for x := 0; x < width; x++ {
		fire[x] = 0
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
				drift := rand.Intn(3) - 1
				dstX := x + drift
				if dstX < 0 {
					dstX = 0
				} else if dstX >= width {
					dstX = width - 1
				}

				dstIndex := (y-1)*width + dstX
				if dstIndex < 0 {
					continue
				}

				dist := math.Abs(float64(x) - center)
				normDist := dist / (halfWidth * 0.8) // Reverted to previous width

				// Slower decay for a larger, taller fire
				decay := 1 + int(normDist*normDist*6.0)

				if y < fireHeight/2 { // Heat carries further up
					// Occasionally reduce decay to let "licks" of flame go higher
					if rand.Float64() > 0.8 {
						decay = 0
					} else {
						decay += 1
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

	// 2. Stable Refuel
	minLX, maxLX := width, 0
	for x := 0; x < width; x++ {
		if getLogHeight(x) > 0 {
			if x < minLX {
				minLX = x
			}
			if x > maxLX {
				maxLX = x
			}
		}
	}

	logSpan := float64(maxLX - minLX)
	fireSpan := logSpan * 0.8
	fireCenter := float64(minLX+maxLX) / 2.0

	for x := 0; x < width; x++ {
		h := getLogHeight(x)
		if h <= 0 {
			continue
		}

		dist := math.Abs(float64(x) - fireCenter)
		// Only refuel within the 80% span
		if dist > fireSpan/2.0 {
			continue
		}

		normDist := dist / (fireSpan / 2.0)

		if rand.Float64() > normDist*0.9 {
			// Inject heat at various depths within logs
			for i := 0; i < 3; i++ { // More heat sources
				// Fire extends higher into the bundle
				d := rand.Intn(h*3/4 + 1)
				fireY := (height - 1 - d) * 2
				if fireY >= 0 && fireY < fireHeight {
					fire[fireY*width+x] = 36
				}
			}
		}
	}
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func drawFireBlended() {
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			sy1 := y * 2
			sy2 := y*2 + 1

			if sy2*width+x >= len(fire) {
				continue
			}

			heat1 := fire[sy1*width+x]
			heat2 := fire[sy2*width+x]

			// Only process if there is actual heat to display
			if heat1 < 4 && heat2 < 4 {
				continue
			}

			// Get existing color from the sticks
			_, _, existingStyle, _ := screen.GetContent(x, y)
			existingFg, existingBg, _ := existingStyle.Decompose()

			// FIX: Ensure we don't blend with the terminal's default white/grey
			// If it's the default foreground/background, treat it as black
			if existingFg == tcell.ColorWhite || existingFg == tcell.ColorDefault {
				existingFg = tcell.ColorBlack
			}
			if existingBg == tcell.ColorWhite || existingBg == tcell.ColorDefault {
				existingBg = tcell.ColorBlack
			}

			// Map heat to fire colors
			fireC1 := colors[clamp(heat1)]
			fireC2 := colors[clamp(heat2)]

			// Blend fire colors with existing stick/background colors
			c1 := blendColors(existingFg, fireC1, heat1)
			c2 := blendColors(existingBg, fireC2, heat2)

			style := tcell.StyleDefault.Foreground(c1).Background(c2)
			screen.SetContent(x, y, '▀', nil, style)
		}
	}
}

func blendColors(base, overlay tcell.Color, heat int) tcell.Color {
	// If no heat, return the base (wood or black)
	if heat <= 0 {
		return base
	}

	br, bg, bb := base.RGB()
	or, og, ob := overlay.RGB()

	// Use heat as the blend factor
	alpha := float64(heat) / 36.0 
	
	// Ensure high heat doesn't blow out to white by capping the intensity
	if alpha > 0.8 {
		alpha = 0.8
	}

	r := int32(float64(br)*(1.0-alpha) + float64(or)*alpha)
	g := int32(float64(bg)*(1.0-alpha) + float64(og)*alpha)
	b := int32(float64(bb)*(1.0-alpha) + float64(ob)*alpha)

	return tcell.NewRGBColor(clampColor(r), clampColor(g), clampColor(b))
}

func drawEnvironment(minID, maxID int) {
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			logID := 0
			if x >= 0 && x < width && y >= 0 && y < height {
				logID = woodMap[y*width+x]
			}

			if logID >= minID && logID <= maxID {
				depth := float64(logID) / float64(logCount)

				// Base stick colors (dark browns)
				br := int32(25 + depth*35)
				bg := int32(15 + depth*20)
				bb := int32(10 + depth*10)

				// Get local fire heat for glow
				heat1 := 0
				heat2 := 0
				if y*2 < fireHeight {
					heat1 = fire[(y*2)*width+x]
				}
				if y*2+1 < fireHeight {
					heat2 = fire[(y*2+1)*width+x]
				}
				avgHeat := (heat1 + heat2) / 2

				// Add fire glow to the stick
				r := br + int32(avgHeat*5)
				g := bg + int32(avgHeat*2)
				b := bb

				baseColor := tcell.NewRGBColor(clampColor(r), clampColor(g), clampColor(b))
				darkColor := tcell.NewRGBColor(clampColor(r/2), clampColor(g/2), clampColor(b/2))

				noise := (x*13 + y*37 + logID*7) % 10
				var style tcell.Style

				// Texture characters
				chars := []rune{' ', ' ', '.', ',', '\'', '`', '.', ' ', ' ', ' '}
				char := chars[noise%len(chars)]

				if noise > 5 {
					style = tcell.StyleDefault.Background(darkColor).Foreground(baseColor)
				} else {
					style = tcell.StyleDefault.Background(baseColor).Foreground(darkColor)
				}

				screen.SetContent(x, y, char, nil, style)
			}
		}
	}
}

func clampColor(v int32) int32 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func clampFloat32(v, min, max int32) int32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
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
