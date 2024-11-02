package main

import (
	"bytes"
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

const (
	screenWidth  = 640
	screenHeight = 480
	gridSize     = 20
)

type Direction int

const (
	Right Direction = iota
	Left
	Up
	Down
)

type Position struct {
	X int
	Y int
}

type Particle struct {
	x, y   float64    // Position
	dx, dy float64    // Velocity/Direction
	life   int        // How long particle lives
	color  color.RGBA // Particle color
	size   float64    // Particle size
}

const (
	particleLife = 30 // How many frames a particle lives
	numParticles = 30 // Number of particles per explosion
)

type Game struct {
	snakeBody     []Position
	direction     Direction
	lastMove      time.Time
	moveSpeed     time.Duration
	grow          bool
	foodPos       Position
	gameOver      bool
	score         int
	otoCtx        *oto.Context
	eatSoundData  []byte
	gameOverSound []byte
	bgMusicData   []byte
	bgMusicPlayer *oto.Player
	particles     []Particle
}

func (g *Game) placeFood() {
	var newPos Position
	cellX := rand.Intn(screenWidth / gridSize)
	cellY := rand.Intn(screenHeight / gridSize)

	newPos = Position{
		X: cellX * gridSize,
		Y: cellY * gridSize,
	}

	g.foodPos = newPos
}

// Restart game function
func (g *Game) restart() {
	// Reset snake to initial position
	g.snakeBody = []Position{
		{screenWidth / 2, screenHeight / 2},
		{screenWidth/2 - gridSize, screenHeight / 2},
		{screenWidth/2 - gridSize*2, screenHeight / 2},
	}
	g.particles = make([]Particle, 0)
	g.direction = Right
	g.score = 0
	g.gameOver = false
	g.placeFood()
}

// Function to check if the snake colloides with self
func (g *Game) checkSelfCollison(pos Position) bool {
	for i := 0; i < len(g.snakeBody); i++ {
		bodyPart := g.snakeBody[i]
		if pos.X == bodyPart.X && pos.Y == bodyPart.Y {
			fmt.Println("Collison detected")
			return true
		}
	}
	return false
}

func (g *Game) createExplosion(x, y float64) {
	for i := 0; i < numParticles; i++ {
		// Calculate angle for circular distribution
		angle := rand.Float64() * 2 * math.Pi
		speed := rand.Float64() * 1.25

		particle := Particle{
			x:    x,
			y:    y,
			dx:   math.Cos(angle) * speed, // Spread horizontally
			dy:   math.Sin(angle) * speed, // Spread vertically
			life: particleLife,
			color: color.RGBA{
				R: 255,
				G: 215,
				B: 0,
				A: 255,
			},
			size: rand.Float64()*3 + 1,
		}
		g.particles = append(g.particles, particle)
	}
}

func (g *Game) Update() error {

	if g.gameOver {
		if ebiten.IsKeyPressed(ebiten.KeySpace) {
			g.restart()
		}
		return nil
	}

	if ebiten.IsKeyPressed(ebiten.KeyRight) && g.direction != Left {
		g.direction = Right
	} else if ebiten.IsKeyPressed(ebiten.KeyLeft) && g.direction != Right {
		g.direction = Left
	} else if ebiten.IsKeyPressed(ebiten.KeyDown) && g.direction != Up {
		g.direction = Down
	} else if ebiten.IsKeyPressed(ebiten.KeyUp) && g.direction != Down {
		g.direction = Up
	}

	if time.Since(g.lastMove) > g.moveSpeed {
		head := g.snakeBody[0]

		newHead := Position{X: head.X, Y: head.Y}
		switch g.direction {
		case Right:
			newHead.X += gridSize
		case Left:
			newHead.X -= gridSize
		case Up:
			newHead.Y -= gridSize
		case Down:
			newHead.Y += gridSize
		}

		// Wrap-around logic
		if newHead.X < 0 {
			newHead.X = screenWidth - gridSize
		} else if newHead.X > screenWidth {
			newHead.X = 0
		}

		if newHead.Y < 0 {
			newHead.Y = screenHeight - gridSize
		} else if newHead.Y > screenHeight {
			newHead.Y = 0
		}

		if g.checkSelfCollison(newHead) {
			g.gameOver = true
			go g.playGameOverSound()
			return nil
		}

		if newHead.X == g.foodPos.X && newHead.Y == g.foodPos.Y {
			g.grow = true
			// Create explosion at food position
			go g.createExplosion(float64(g.foodPos.X), float64(g.foodPos.Y))
			g.placeFood()
			g.score += 10
			go g.playEatSound()
		}

		if !g.grow {
			// remove last item from the body / remove tail
			g.snakeBody = g.snakeBody[:len(g.snakeBody)-1]
		} else {
			g.grow = false
		}
		// Update the head with newHead
		g.snakeBody = append([]Position{newHead}, g.snakeBody...)

		g.lastMove = time.Now()
	}
	
	var activeParticles []Particle
	for _, p := range g.particles {
		if p.life > 0 {
			p.x += p.dx
			p.y += p.dy
			p.life--
			// Fade out effect
			p.color.A = uint8(float64(p.life) / float64(particleLife) * 255)
			activeParticles = append(activeParticles, p)
		}
	}
	g.particles = activeParticles

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	var bgColor color.RGBA
	if g.gameOver {
		// Redish bg color
		bgColor = color.RGBA{255, 0, 0, 255}
	} else {
		bgColor = color.RGBA{40, 40, 40, 255}
	}
	screen.Fill(bgColor)
	for i, p := range g.snakeBody {
		segment := ebiten.NewImage(gridSize-1, gridSize-1)
		// we will make the head brigher and body little lighter
		if i == 0 {
			segment.Fill(color.RGBA{0, 255, 0, 255})
		} else {
			segment.Fill(color.RGBA{0, 150, 0, 150})
		}
		opt := &ebiten.DrawImageOptions{}
		opt.GeoM.Translate(float64(p.X), float64(p.Y))
		screen.DrawImage(segment, opt)
	}

	// draw food
	food := ebiten.NewImage(gridSize, gridSize)
	foodOpt := &ebiten.DrawImageOptions{}
	foodOpt.GeoM.Translate(float64(g.foodPos.X), float64(g.foodPos.Y))
	// red color for food
	food.Fill(color.RGBA{255, 0, 0, 1})
	screen.DrawImage(food, foodOpt)

	score := fmt.Sprintf("Score: %d", g.score)
	text.Draw(screen, score, basicfont.Face7x13, 10, 20, color.White)

	if g.gameOver {
		gameOverText := "Game Over! Press Space to restart"
		text.Draw(screen, gameOverText, basicfont.Face7x13, screenWidth/2-len(gameOverText)*3, screenHeight/2, color.White)
	}

	for _, p := range g.particles {
		particle := ebiten.NewImage(int(p.size), int(p.size))
		particle.Fill(p.color)

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(p.x, p.y)
		screen.DrawImage(particle, op)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 640, 480
}

func (g *Game) playEatSound() {
	reader := bytes.NewReader(g.eatSoundData)
	decoder, err := mp3.NewDecoder(reader)
	if err != nil {
		fmt.Println("Error decoding mp3:", err)
		return
	}

	player := g.otoCtx.NewPlayer(decoder)
	player.SetVolume(0.1)
	player.Play()

	// Wait for the sound to finish playing
	for player.IsPlaying() {
		time.Sleep(time.Millisecond)
	}
	player.Close()
}

func (g *Game) playGameOverSound() {
	reader := bytes.NewReader(g.gameOverSound)
	decoder, err := mp3.NewDecoder(reader)
	if err != nil {
		fmt.Println("Error decoding game over sound:", err)
		return
	}

	player := g.otoCtx.NewPlayer(decoder)
	player.Play()

	// Wait for the sound to finish playing
	for player.IsPlaying() {
		time.Sleep(time.Millisecond)
	}
	player.Close()
}

// func (g *Game) stopBackgroundMusic() {
//     if g.bgMusicPlayer != nil {
//         g.bgMusicPlayer.Close()
//         g.bgMusicPlayer = nil
//     }
// }

// func (g *Game) pauseBackgroundMusic() {
//     if g.bgMusicPlayer != nil {
//         g.bgMusicPlayer.Pause()
//     }
// }

// func (g *Game) resumeBackgroundMusic() {
//     if g.bgMusicPlayer != nil {
//         g.bgMusicPlayer.Play()
//     }
// }

func (g *Game) startBackgroundMusic() {
	for {
		reader := bytes.NewReader(g.bgMusicData)
		decoder, err := mp3.NewDecoder(reader)
		if err != nil {
			fmt.Println("Error decoding background music:", err)
			return
		}

		player := g.otoCtx.NewPlayer(decoder)
		player.SetVolume(0.1)
		g.bgMusicPlayer = player

		player.Play()

		for player.IsPlaying() {
			time.Sleep(time.Millisecond)
		}

		player.Close()
	}
}

func main() {
	op := &oto.NewContextOptions{
		SampleRate:   44100,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
		BufferSize:   4096,
	}

	otoCtx, readyChan, err := oto.NewContext(op)
	if err != nil {
		log.Fatal("error creating audio context:", err)
	}
	<-readyChan

	// Read eat sound file
	eatSoundData, err := os.ReadFile("eat-food.mp3")
	if err != nil {
		log.Fatal("error reading eat sound file:", err)
	}

	// Read game over sound file
	gameOverSound, err := os.ReadFile("game-over.mp3")
	if err != nil {
		log.Fatal("error reading game over sound file:", err)
	}

	bgMusicData, err := os.ReadFile("background.mp3")
	if err != nil {
		log.Fatal("error reading background music file:", err)
	}

	game := &Game{
		snakeBody: []Position{
			{X: 300, Y: 240},
			{X: 280, Y: 240},
			{X: 260, Y: 240},
		},
		direction:     Right,
		lastMove:      time.Now(),
		moveSpeed:     100 * time.Millisecond,
		score:         0,
		otoCtx:        otoCtx,
		eatSoundData:  eatSoundData,
		gameOverSound: gameOverSound,
		bgMusicData:   bgMusicData,
		particles:     make([]Particle, 0),
	}

	game.placeFood()
	go game.startBackgroundMusic()

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Snake Game")
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

func (g *Game) Close() {
	if g.bgMusicPlayer != nil {
		g.bgMusicPlayer.Close()
	}
}

