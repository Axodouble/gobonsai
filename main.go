package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

// BranchType represents different types of branches
type BranchType int

const (
	Trunk BranchType = iota
	ShootLeft
	ShootRight
	Dying
	Dead
)

// Config holds all configuration options
type Config struct {
	Live       bool
	Infinite   bool
	PrintTree  bool
	LifeStart  int
	Multiplier int
	BaseType   int
	Seed       int64
	TimeStep   float64
	TimeWait   float64
	Message    string
	Leaves     []string
	Width      int
	Height     int
	UseColors  bool
}

// Color constants for ANSI escape codes
const (
	ColorReset = "\033[0m"
	ColorBold  = "\033[1m"

	// Text colors
	ColorBlack   = "\033[30m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorWhite   = "\033[37m"

	// Bright colors
	ColorBrightBlack   = "\033[90m"
	ColorBrightRed     = "\033[91m"
	ColorBrightGreen   = "\033[92m"
	ColorBrightYellow  = "\033[93m"
	ColorBrightBlue    = "\033[94m"
	ColorBrightMagenta = "\033[95m"
	ColorBrightCyan    = "\033[96m"
	ColorBrightWhite   = "\033[97m"

	// 256-color support
	ColorBrown       = "\033[38;5;94m"  // Brown for branches
	ColorDarkBrown   = "\033[38;5;52m"  // Dark brown for trunk
	ColorLightBrown  = "\033[38;5;130m" // Light brown for branches
	ColorDarkGreen   = "\033[38;5;22m"  // Dark green for leaves
	ColorMediumGreen = "\033[38;5;28m"  // Medium green for leaves
	ColorTerracotta  = "\033[38;5;166m" // Terracotta for pot
	ColorOrange      = "\033[38;5;214m" // Orange/autumn leaves
)

// Point represents a coordinate
type Point struct {
	X, Y int
}

// BonsaiTree represents the tree structure
type BonsaiTree struct {
	canvas        [][]rune
	colorCanvas   [][]string // Store color for each character
	config        *Config
	branches      int
	shoots        int
	rng           *rand.Rand
	initialized   bool
	messageOffset int
}

// Terminal size detection
func getTerminalSize() (int, int) {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		fmt.Printf("Error getting terminal size: %v\n", err)
		return 80, 24 // Default fallback
	}
	return width, height
}

func saveConsole() {
	fmt.Print("\033[s")    // Save cursor position
	fmt.Print("\033[?47h") // Switch to alternate screen buffer
}

func restoreConsole() {
	fmt.Print("\033[?47l") // Switch back to normal screen buffer
	fmt.Print("\033[u")    // Restore cursor position
}

// setupSignalHandler sets up a signal handler to restore console on interrupt
func setupSignalHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		restoreConsole()
		fmt.Print("\033[?25h") // Show cursor
		os.Exit(0)
	}()
}

// NewBonsaiTree creates a new bonsai tree
func NewBonsaiTree(config *Config) *BonsaiTree {
	width, height := getTerminalSize()
	config.Width = width
	config.Height = height

	canvas := make([][]rune, height)
	colorCanvas := make([][]string, height)
	for i := range canvas {
		canvas[i] = make([]rune, width)
		colorCanvas[i] = make([]string, width)
		for j := range canvas[i] {
			canvas[i][j] = ' '
			colorCanvas[i][j] = ""
		}
	}

	return &BonsaiTree{
		canvas:        canvas,
		colorCanvas:   colorCanvas,
		config:        config,
		rng:           rand.New(rand.NewSource(config.Seed)),
		initialized:   false,
		messageOffset: 0,
	}
}

// GetBranchColor returns the appropriate color for branch types
func (bt *BonsaiTree) GetBranchColor(branchType BranchType) string {
	if !bt.config.UseColors {
		return ""
	}

	switch branchType {
	case Trunk:
		return ColorYellow
	case ShootLeft, ShootRight:
		// Lighter browns for smaller branches
		if bt.rng.Intn(4) == 0 {
			return ColorYellow
		}
		return ColorBrightYellow
	case Dying, Dead:
		// Green leaves with occasional brown/yellow
		dice := bt.rng.Intn(10)
		switch {
		case dice <= 8:
			return ColorBrightGreen // Some darker green
		case dice == 9:
			return ColorMediumGreen // Some yellow/autumn leaves
		case dice == 9:
			return ColorBrightYellow // Some brown/dead leaves
		}
	}
	return ""
}

// GetBaseColor returns the appropriate color for the pot/base
func (bt *BonsaiTree) GetBaseColor() string {
	if !bt.config.UseColors {
		return ""
	}
	return ColorBrightBlack
}

// MoveCursor moves cursor to specific position (1-based coordinates)
func (bt *BonsaiTree) MoveCursor(x, y int) {
	fmt.Printf("\033[%d;%dH", y, x)
}

// ClearScreen clears the screen using the alternate buffer (preserves original console)
func (bt *BonsaiTree) ClearScreen() {
	// Only clear screen for interactive modes, not for print mode
	if !bt.config.PrintTree {
		// Clear the alternate screen buffer
		fmt.Print("\033[2J") // Clear entire screen
		fmt.Print("\033[H")  // Move cursor to top-left
	}
}

// SetPixelLive sets a character at the given position and immediately renders it in live mode
func (bt *BonsaiTree) SetPixelLive(x, y int, char rune, color string) {
	if y >= 0 && y < len(bt.canvas) && x >= 0 && x < len(bt.canvas[y]) {
		bt.canvas[y][x] = char
		if bt.config.Live {
			bt.MoveCursor(x+1, y+1) // Convert to 1-based coordinates
			if color != "" {
				fmt.Printf("%s%c%s", color, char, ColorReset)
			} else {
				fmt.Printf("%c", char)
			}
			os.Stdout.Sync() // Ensure immediate output
		}
	}
}

// SetPixel sets a character at the given position
func (bt *BonsaiTree) SetPixel(x, y int, char rune, color string) {
	if y >= 0 && y < len(bt.canvas) && x >= 0 && x < len(bt.canvas[y]) {
		bt.canvas[y][x] = char
		bt.colorCanvas[y][x] = color
	}
}

// GetDeltas calculates movement deltas based on branch type and age
func (bt *BonsaiTree) GetDeltas(branchType BranchType, life, age int) (int, int) {
	dx, dy := 0, 0

	switch branchType {
	case Trunk:
		if age <= 2 || life < 4 {
			dy = 0
			dx = bt.rng.Intn(3) - 1
		} else if age < (bt.config.Multiplier * 3) {
			if age%(int(float64(bt.config.Multiplier)*0.5+0.5)) == 0 {
				dy = -1
			} else {
				dy = 0
			}
			dice := bt.rng.Intn(10)
			switch {
			case dice == 0:
				dx = -2
			case dice <= 3:
				dx = -1
			case dice <= 5:
				dx = 0
			case dice <= 8:
				dx = 1
			case dice == 9:
				dx = 2
			}
		} else {
			if bt.rng.Intn(10) > 2 {
				dy = -1
			} else {
				dy = 0
			}
			dx = bt.rng.Intn(3) - 1
		}

	case ShootLeft:
		dice := bt.rng.Intn(10)
		switch {
		case dice <= 1:
			dy = -1
		case dice <= 7:
			dy = 0
		default:
			dy = 1
		}
		dice = bt.rng.Intn(10)
		switch {
		case dice <= 1:
			dx = -2
		case dice <= 5:
			dx = -1
		case dice <= 8:
			dx = 0
		default:
			dx = 1
		}

	case ShootRight:
		dice := bt.rng.Intn(10)
		switch {
		case dice <= 1:
			dy = -1
		case dice <= 7:
			dy = 0
		default:
			dy = 1
		}
		dice = bt.rng.Intn(10)
		switch {
		case dice <= 1:
			dx = 2
		case dice <= 5:
			dx = 1
		case dice <= 8:
			dx = 0
		default:
			dx = -1
		}

	case Dying:
		dice := bt.rng.Intn(10)
		switch {
		case dice <= 1:
			dy = -1
		case dice <= 8:
			dy = 0
		default:
			dy = 1
		}
		dice = bt.rng.Intn(15)
		switch {
		case dice == 0:
			dx = -3
		case dice <= 2:
			dx = -2
		case dice <= 5:
			dx = -1
		case dice <= 8:
			dx = 0
		case dice <= 11:
			dx = 1
		case dice <= 13:
			dx = 2
		default:
			dx = 3
		}

	case Dead:
		dice := bt.rng.Intn(10)
		switch {
		case dice <= 2:
			dy = -1
		case dice <= 6:
			dy = 0
		default:
			dy = 1
		}
		dx = bt.rng.Intn(3) - 1
	}

	return dx, dy
}

// ChooseChar selects the appropriate character for the branch
func (bt *BonsaiTree) ChooseChar(branchType BranchType, life, dx, dy int) rune {
	if life < 4 {
		branchType = Dying
	}

	switch branchType {
	case Trunk:
		if dy == 0 {
			return '~'
		} else if dx < 0 {
			return '\\'
		} else if dx == 0 {
			return '|'
		} else {
			return '/'
		}

	case ShootLeft:
		if dy > 0 {
			return '\\'
		} else if dy == 0 {
			return '_'
		} else if dx < 0 {
			return '\\'
		} else if dx == 0 {
			return '|'
		} else {
			return '/'
		}

	case ShootRight:
		if dy > 0 {
			return '/'
		} else if dy == 0 {
			return '_'
		} else if dx < 0 {
			return '\\'
		} else if dx == 0 {
			return '|'
		} else {
			return '/'
		}

	case Dying, Dead:
		if len(bt.config.Leaves) > 0 {
			return rune(bt.config.Leaves[bt.rng.Intn(len(bt.config.Leaves))][0])
		}
		return '&'
	}

	return '?'
}

// Branch generates a branch recursively
func (bt *BonsaiTree) Branch(x, y int, branchType BranchType, life int) {
	bt.branches++
	shootCooldown := bt.config.Multiplier

	for life > 0 {
		life--
		age := bt.config.LifeStart - life

		dx, dy := bt.GetDeltas(branchType, life, age)

		// Prevent going too close to ground
		if dy > 0 && y > (bt.config.Height-7) {
			dy--
		}
		// Ensure first move is at least 1 up
		if age == 1 && dy >= 0 {
			dy = -2
		}

		// Branching logic
		if life < 3 {
			bt.Branch(x, y, Dead, life)
		} else if branchType == Trunk && life < (bt.config.Multiplier+2) {
			bt.Branch(x, y, Dying, life)
		} else if (branchType == ShootLeft || branchType == ShootRight) && life < (bt.config.Multiplier+2) {
			bt.Branch(x, y, Dying, life)
		} else if branchType == Trunk && (bt.rng.Intn(3) == 0 || life%bt.config.Multiplier == 0) {
			if bt.rng.Intn(8) == 0 && life > 7 {
				shootCooldown = bt.config.Multiplier * 2
				bt.Branch(x, y, Trunk, life+bt.rng.Intn(5)-2)
			} else if shootCooldown <= 0 {
				shootCooldown = bt.config.Multiplier * 2
				shootLife := life + bt.config.Multiplier
				bt.shoots++
				if bt.shoots%2 == 0 {
					bt.Branch(x, y, ShootLeft, shootLife)
				} else {
					bt.Branch(x, y, ShootRight, shootLife)
				}
			}
		}
		shootCooldown--

		// Move and draw
		x += dx
		y += dy

		char := bt.ChooseChar(branchType, life, dx, dy)
		color := bt.GetBranchColor(branchType)
		if bt.config.Live {
			bt.SetPixelLive(x, y, char, color)
		} else {
			bt.SetPixel(x, y, char, color)
		}

		// Live mode animation
		if bt.config.Live {
			time.Sleep(time.Duration(bt.config.TimeStep * float64(time.Second)))
		}
	}
}

// DrawBase draws the base of the tree
func (bt *BonsaiTree) DrawBase() {
	if bt.config.BaseType == 0 {
		return
	}

	baseY := bt.config.Height - 1
	centerX := bt.config.Width / 2
	grassColor := ""
	if bt.config.UseColors {
		grassColor = ColorBrightGreen
	}

	switch bt.config.BaseType {
	case 1:
		// Draw grass/foliage line first (within the pot boundaries)
		// Calculate pot boundaries based on the base line

		line3 := ":___________./~~~\\.___________:"
		startX := centerX - len(line3)/2
		baseColor := bt.GetBaseColor()

		// Draw the pot markers (^) in gray/base color and grass in between
		for i, char := range line3 {
			var currentColor string
			if char == '(' || char == ')' || char == '^' {
				// Pot markers in gray/base color
				currentColor = baseColor
			} else if char == '_' {
				// Replace '_' with a grass character and make it green
				grassChars := ".,~`'^\"*o%"
				index := ((i - 7) + len(grassChars)) % len(grassChars)
				char = rune(grassChars[index])
				currentColor = grassColor
			} else if char == '.' || char == '/' || char == '~' || char == '\\' {
				// Turn ., /, ~ brown
				currentColor = ColorYellow
			} else if i > 0 && i < 30 { // Between the (^) markers
				// Grass in the middle section
				if char == ' ' {
					// Replace spaces with grass characters
					grassChars := ".,~`'^\"*o%"
					char = rune(grassChars[(i-7)%len(grassChars)])
				}
				currentColor = grassColor
			} else {
				// Spaces outside - keep as base color
				currentColor = baseColor
			}

			if bt.config.Live {
				bt.SetPixelLive(startX+i, baseY-3, char, currentColor)
			} else {
				bt.SetPixel(startX+i, baseY-3, char, currentColor)
			}
		}

		line2 := " \\                           / "
		startX = centerX - len(line2)/2
		for i, char := range line2 {
			if bt.config.Live {
				bt.SetPixelLive(startX+i, baseY-2, char, baseColor)
			} else {
				bt.SetPixel(startX+i, baseY-2, char, baseColor)
			}
		}

		line1 := "  \\_________________________/ "
		startX = centerX - len(line1)/2
		for i, char := range line1 {
			if bt.config.Live {
				bt.SetPixelLive(startX+i, baseY-1, char, baseColor)
			} else {
				bt.SetPixel(startX+i, baseY-1, char, baseColor)
			}
		}

		base := "   (^)                 (^)   "
		startX = centerX - len(base)/2
		for i, char := range base {
			if bt.config.Live {
				bt.SetPixelLive(startX+i, baseY, char, baseColor)
			} else {
				bt.SetPixel(startX+i, baseY, char, baseColor)
			}
		}

	case 2:
		// Draw grass/foliage line first (above the pot)
		grassLine := ".,~`'^\".,~`'^\".,~`'^\"."
		grassStartX := centerX - len(grassLine)/2
		grassColor := ""
		if bt.config.UseColors {
			grassColor = ColorMediumGreen
		}
		for i, char := range grassLine {
			if grassStartX+i >= 0 && grassStartX+i < bt.config.Width && baseY-3 >= 0 {
				if bt.config.Live {
					bt.SetPixelLive(grassStartX+i, baseY-3, char, grassColor)
				} else {
					bt.SetPixel(grassStartX+i, baseY-3, char, grassColor)
				}
			}
		}

		line2 := "  (^^^^^^^)  "
		startX := centerX - len(line2)/2
		for i, char := range line2 {
			// Color the parentheses and ^ as grass/green
			currentColor := grassColor
			if bt.config.Live {
				bt.SetPixelLive(startX+i, baseY-2, char, currentColor)
			} else {
				bt.SetPixel(startX+i, baseY-2, char, currentColor)
			}
		}

		line1 := " (           ) "
		startX = centerX - len(line1)/2
		baseColor := bt.GetBaseColor()
		for i, char := range line1 {
			if bt.config.Live {
				bt.SetPixelLive(startX+i, baseY-1, char, baseColor)
			} else {
				bt.SetPixel(startX+i, baseY-1, char, baseColor)
			}
		}

		base := "(---./~~~\\.---)"
		startX = centerX - len(base)/2
		for i, char := range base {
			if bt.config.Live {
				bt.SetPixelLive(startX+i, baseY, char, baseColor)
			} else {
				bt.SetPixel(startX+i, baseY, char, baseColor)
			}
		}
	}
}

// Render displays the current state of the tree
func (bt *BonsaiTree) Render() {
	if !bt.initialized {
		bt.ClearScreen()
		bt.initialized = true
	}

	// Only render the full screen if not in live mode
	if !bt.config.Live {
		// For print mode, don't use cursor positioning
		if bt.config.PrintTree {
			for y := 0; y < len(bt.canvas); y++ {
				for x := 0; x < len(bt.canvas[y]); x++ {
					char := bt.canvas[y][x]
					color := bt.colorCanvas[y][x]
					if color != "" && bt.config.UseColors {
						fmt.Printf("%s%c%s", color, char, ColorReset)
					} else {
						fmt.Printf("%c", char)
					}
				}
				fmt.Println()
			}
			if bt.config.Message != "" {
				fmt.Printf("\n%s\n", bt.config.Message)
			}
		} else {
			// For interactive mode, use cursor positioning
			bt.MoveCursor(1, 1)
			for y := 0; y < len(bt.canvas); y++ {
				for x := 0; x < len(bt.canvas[y]); x++ {
					char := bt.canvas[y][x]
					color := bt.colorCanvas[y][x]
					if color != "" && bt.config.UseColors {
						fmt.Printf("%s%c%s", color, char, ColorReset)
					} else {
						fmt.Printf("%c", char)
					}
				}
				fmt.Println()
			}
			if bt.config.Message != "" {
				fmt.Printf("\n%s\n", bt.config.Message)
			}
		}
	} else {
		// In live mode, render message if it hasn't been rendered yet
		if bt.config.Message != "" && bt.messageOffset == 0 {
			bt.MoveCursor(1, bt.config.Height+2)
			fmt.Printf("%s", bt.config.Message)
			bt.messageOffset = len(bt.config.Message)
		}
	}
}

// GrowTree generates the complete tree
func (bt *BonsaiTree) GrowTree() {
	bt.branches = 0
	bt.shoots = 0
	bt.initialized = false
	bt.messageOffset = 0

	// Clear canvas
	for i := range bt.canvas {
		for j := range bt.canvas[i] {
			bt.canvas[i][j] = ' '
			bt.colorCanvas[i][j] = ""
		}
	}

	// Initialize screen for live mode
	if bt.config.Live {
		bt.ClearScreen()
		bt.initialized = true
	}

	bt.DrawBase()

	startX := bt.config.Width / 2
	startY := bt.config.Height + 2
	if bt.config.BaseType > 0 {
		startY -= 5 // Account for base height + grass line above the pot
	}

	bt.Branch(startX, startY, Trunk, bt.config.LifeStart)

	if !bt.config.Live {
		bt.Render()
	}
}

func main() {
	config := &Config{
		Live:       false,
		Infinite:   false,
		PrintTree:  false,
		LifeStart:  32,
		Multiplier: 5,
		BaseType:   1,
		Seed:       0,
		TimeStep:   0.03,
		TimeWait:   4.0,
		Message:    "",
		Leaves:     []string{"&", "*", "o", "@", "%"},
		UseColors:  true, // Enable colors by default
	}

	// Parse command line flags
	flag.BoolVar(&config.Live, "live", false, "Live mode: show each step of growth")
	flag.BoolVar(&config.Live, "l", false, "Live mode: show each step of growth")
	flag.BoolVar(&config.Infinite, "infinite", false, "Infinite mode: keep growing trees")
	flag.BoolVar(&config.Infinite, "i", false, "Infinite mode: keep growing trees")
	flag.BoolVar(&config.PrintTree, "print", false, "Print tree to terminal when finished")
	flag.BoolVar(&config.PrintTree, "p", false, "Print tree to terminal when finished")
	flag.IntVar(&config.LifeStart, "life", 32, "Life: higher -> more growth (0-200)")
	flag.IntVar(&config.LifeStart, "L", 32, "Life: higher -> more growth (0-200)")
	flag.IntVar(&config.Multiplier, "multiplier", 5, "Branch multiplier: higher -> more branching (0-20)")
	flag.IntVar(&config.Multiplier, "M", 5, "Branch multiplier: higher -> more branching (0-20)")
	flag.IntVar(&config.BaseType, "base", 1, "ASCII-art plant base to use, 0 is none")
	flag.IntVar(&config.BaseType, "b", 1, "ASCII-art plant base to use, 0 is none")
	flag.Float64Var(&config.TimeStep, "time", 0.03, "In live mode, wait TIME secs between steps")
	flag.Float64Var(&config.TimeStep, "t", 0.03, "In live mode, wait TIME secs between steps")
	flag.Float64Var(&config.TimeWait, "wait", 4.0, "In infinite mode, wait TIME between each tree")
	flag.Float64Var(&config.TimeWait, "w", 4.0, "In infinite mode, wait TIME between each tree")
	flag.StringVar(&config.Message, "message", "", "Attach message next to the tree")
	flag.StringVar(&config.Message, "m", "", "Attach message next to the tree")
	flag.BoolVar(&config.UseColors, "color", true, "Use colors (green leaves, brown branches, colored pot)")
	flag.BoolVar(&config.UseColors, "C", true, "Use colors (green leaves, brown branches, colored pot)")

	var noColor bool
	flag.BoolVar(&noColor, "no-color", false, "Disable colors")

	var seedStr string
	var leavesStr string
	flag.StringVar(&seedStr, "seed", "", "Seed random number generator")
	flag.StringVar(&seedStr, "s", "", "Seed random number generator")
	flag.StringVar(&leavesStr, "leaf", "&,*,o,@,%", "List of comma-delimited strings for leaves")
	flag.StringVar(&leavesStr, "c", "&,*,o,@,%", "List of comma-delimited strings for leaves")

	help := flag.Bool("help", false, "Show help")
	flag.BoolVar(help, "h", false, "Show help")

	flag.Parse()

	if *help {
		fmt.Println("gobonsai - A beautifully random bonsai tree generator in Go")
		fmt.Println("\nUsage: gobonsai [OPTIONS]...")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		return
	}

	// Parse seed
	if seedStr != "" {
		if seed, err := strconv.ParseInt(seedStr, 10, 64); err == nil {
			config.Seed = seed
		} else {
			fmt.Printf("Error: invalid seed: %s\n", seedStr)
			os.Exit(1)
		}
	} else {
		config.Seed = time.Now().UnixNano()
	}

	// Parse leaves
	if leavesStr != "" {
		config.Leaves = strings.Split(leavesStr, ",")
	}

	// Handle no-color flag
	if noColor {
		config.UseColors = false
	}

	// Validate configuration
	if config.LifeStart < 0 || config.LifeStart > 200 {
		fmt.Println("Error: life must be between 0 and 200")
		os.Exit(1)
	}
	if config.Multiplier < 0 || config.Multiplier > 20 {
		fmt.Println("Error: multiplier must be between 0 and 20")
		os.Exit(1)
	}
	if config.BaseType < 0 || config.BaseType > 2 {
		fmt.Println("Error: base type must be 0, 1, or 2")
		os.Exit(1)
	}
	if config.TimeStep < 0 {
		fmt.Println("Error: time step must be non-negative")
		os.Exit(1)
	}

	// Hide cursor
	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h") // Show cursor on exit

	// Save console state and setup signal handling (only for interactive modes)
	if !config.PrintTree {
		saveConsole()
		defer restoreConsole()
		setupSignalHandler()
	}

	// Main loop
	for {
		// In infinite mode, generate a new seed for each tree (unless original seed was explicitly set)
		if config.Infinite && seedStr == "" {
			config.Seed = time.Now().UnixNano()
		}

		tree := NewBonsaiTree(config)
		tree.GrowTree()

		if config.PrintTree {
			// Just print and exit
			break
		}

		if config.Infinite {
			time.Sleep(time.Duration(config.TimeWait * float64(time.Second)))
			// Check for interrupt
			exec.Command("stty", "-cbreak", "echo").Run()
		} else {
			tree.MoveCursor(1, tree.config.Height+2)
			fmt.Scanln()
			break
		}
	}
}
