package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
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
}

// Point represents a coordinate
type Point struct {
	X, Y int
}

// BonsaiTree represents the tree structure
type BonsaiTree struct {
	canvas   [][]rune
	config   *Config
	branches int
	shoots   int
	rng      *rand.Rand
}

// Terminal size detection
type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func getTerminalSize() (int, int) {
	ws := &winsize{}
	retCode, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)))

	if int(retCode) == -1 {
		fmt.Printf("Error getting terminal size: %v\n", errno)
		return 80, 24 // Default fallback
	}
	return int(ws.Col), int(ws.Row)
}

// NewBonsaiTree creates a new bonsai tree
func NewBonsaiTree(config *Config) *BonsaiTree {
	width, height := getTerminalSize()
	config.Width = width
	config.Height = height

	canvas := make([][]rune, height)
	for i := range canvas {
		canvas[i] = make([]rune, width)
		for j := range canvas[i] {
			canvas[i][j] = ' '
		}
	}

	return &BonsaiTree{
		canvas: canvas,
		config: config,
		rng:    rand.New(rand.NewSource(config.Seed)),
	}
}

// Clear clears the screen
func (bt *BonsaiTree) Clear() {
	fmt.Print("\033[2J\033[H")
}

// SetPixel sets a character at the given position
func (bt *BonsaiTree) SetPixel(x, y int, char rune) {
	if y >= 0 && y < len(bt.canvas) && x >= 0 && x < len(bt.canvas[y]) {
		bt.canvas[y][x] = char
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
		if dy > 0 && y > (bt.config.Height-3) {
			dy--
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
		bt.SetPixel(x, y, char)

		// Live mode animation
		if bt.config.Live {
			bt.Render()
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

	switch bt.config.BaseType {
	case 1:
		base := ":___________./~~~\\.___________:"
		startX := centerX - len(base)/2
		for i, char := range base {
			bt.SetPixel(startX+i, baseY, char)
		}

		line1 := " \\                           / "
		startX = centerX - len(line1)/2
		for i, char := range line1 {
			bt.SetPixel(startX+i, baseY-1, char)
		}

		line2 := "  \\_________________________/ "
		startX = centerX - len(line2)/2
		for i, char := range line2 {
			bt.SetPixel(startX+i, baseY-2, char)
		}

		line3 := "  (_)                     (_)"
		startX = centerX - len(line3)/2
		for i, char := range line3 {
			bt.SetPixel(startX+i, baseY-3, char)
		}

	case 2:
		base := "(---./~~~\\.---)"
		startX := centerX - len(base)/2
		for i, char := range base {
			bt.SetPixel(startX+i, baseY, char)
		}

		line1 := " (           ) "
		startX = centerX - len(line1)/2
		for i, char := range line1 {
			bt.SetPixel(startX+i, baseY-1, char)
		}

		line2 := "  (_________)  "
		startX = centerX - len(line2)/2
		for i, char := range line2 {
			bt.SetPixel(startX+i, baseY-2, char)
		}
	}
}

// Render displays the current state of the tree
func (bt *BonsaiTree) Render() {
	bt.Clear()

	for y := 0; y < len(bt.canvas); y++ {
		for x := 0; x < len(bt.canvas[y]); x++ {
			fmt.Printf("%c", bt.canvas[y][x])
		}
		fmt.Println()
	}

	if bt.config.Message != "" {
		fmt.Printf("\n%s\n", bt.config.Message)
	}
}

// GrowTree generates the complete tree
func (bt *BonsaiTree) GrowTree() {
	bt.branches = 0
	bt.shoots = 0

	// Clear canvas
	for i := range bt.canvas {
		for j := range bt.canvas[i] {
			bt.canvas[i][j] = ' '
		}
	}

	bt.DrawBase()

	startX := bt.config.Width / 2
	startY := bt.config.Height - 1
	if bt.config.BaseType > 0 {
		startY -= 4 // Account for base height
	}

	bt.Branch(startX, startY, Trunk, bt.config.LifeStart)

	if !bt.config.Live {
		bt.Render()
	}
}

// WaitForKeypress waits for user input
func (bt *BonsaiTree) WaitForKeypress() {
	fmt.Println("\nPress Enter to exit...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
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

	// Seed random number generator
	// rand.New(rand.NewSource(config.Seed)) - now handled in NewBonsaiTree

	// Hide cursor
	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h") // Show cursor on exit

	// Main loop
	for {
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
			tree.WaitForKeypress()
			break
		}
	}
}
