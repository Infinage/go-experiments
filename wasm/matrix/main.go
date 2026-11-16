//go:build wasm && js

package main

import (
	"fmt"
	"math/rand/v2"
	"syscall/js"
	"time"
)

const (
	FONT_SZ_MIN = 10 // 10px
	FONT_SZ_MAX = 20 // 20px
)

func roll(pct float32) bool {
	r := rand.Float32()
	return r < pct
}

type Cell struct {
	start bool
	size int
	char rune
	brightness float32
}

type Board [][]Cell

func (b *Board) Size() (int, int) {
	rows := len(*b)
	if rows == 0 {
		return 0, 0
	}

	cols := len((*b)[0])
	return rows, cols
}

func (b *Board) Resize(h, w int) error {
	if h <= 0 || w <= 0 {
		return fmt.Errorf("negative board dims")
	}

	newBoard := make([][]Cell, h / FONT_SZ_MAX)
	for i := range newBoard {
		newBoard[i] = make([]Cell, w / FONT_SZ_MAX)
	}

	rows, cols := b.Size()
	for i := range min(rows, len(newBoard)) {
		for j := range min(cols, len(newBoard[i])) {
			newBoard[i][j] = (*b)[i][j]
		}
	}

	*b = newBoard
	return nil
}

// step shifts the rows down and randomly inserts new lines 
// at the top. Rain pours and new rain drops continues forming
func (b *Board) Step() {
	rows, cols := b.Size()
	top := make([]Cell, cols)
	for i := range top {
		if roll(0.015) {
			c := '0'
			if roll(0.5) {
				c = '1'
			}
			sz := rand.IntN(FONT_SZ_MAX - FONT_SZ_MIN) + FONT_SZ_MIN
			top[i] = Cell{start: true, size: sz, brightness: 100, char: c}
		}
	}

	for j := range cols {
		// Carry over from drops encountered - this loop runs on fresh iteration
		// so need to ensure this particular starting point is handled.
		var brightness float32
		var size int

		for i := rows - 1; i > 0; i-- {
			// Copy over from cell above
			curr, next := &(*b)[i][j], (*b)[i-1][j]
			*curr = next

			// Flip based on cell's brightness
			if curr.char == 0 || roll(max(0, (curr.brightness - 25)) / 100) {
				if curr.char == '0' {
					curr.char = '1'
				} else {
					curr.char = '0'
				}
			}

			// Next guy is the start of a new drop
			if curr.start {
				brightness, size = curr.brightness, curr.size
				continue
			}

			// Residual energy can sustain current drop
			brightness /= 1.35
			if brightness > 5 {
				curr.brightness, curr.size = brightness, size
				continue
			}

			// No energy left
			if curr.brightness < 5 {
				curr.char = 0
			}
		}

		// Copy from top row for the very first row
		(*b)[0][j] = top[j]
	}
}

func (b *Board) Draw(canvas js.Value, ctx js.Value) {
	h, w := canvas.Get("height").Int(), canvas.Get("width").Int()
	ctx.Call("clearRect", 0, 0, w, h)

	for i, row := range *b {
		for j, cell := range row {
			if cell.char == 0 {
				continue
			}

			ctx.Set("font", fmt.Sprintf("%dpx serif", cell.size))
			ctx.Set("fillStyle", fmt.Sprintf("hsl(144,100%%,%.2f%%)", cell.brightness))
			ctx.Call("fillText", string(cell.char), j * FONT_SZ_MAX, i * FONT_SZ_MAX)
		}
	}
}

func main() {
	var b Board

	// Init canvas size to height / width of page
	win := js.Global().Get("window")
	canvas := js.Global().Get("document").Call("querySelector", "canvas")
	h, w := win.Get("innerHeight").Int(), win.Get("innerWidth").Int()
	canvas.Call("setAttribute", "height", h)
	canvas.Call("setAttribute", "width", w)
	_ = b.Resize(h, w)

	// Update canvas size on window resize event
	win.Call("addEventListener", "resize", js.FuncOf(func(this js.Value, args []js.Value) any {
		h, w = win.Get("innerHeight").Int(), win.Get("innerWidth").Int()
		canvas.Call("setAttribute", "height", h)
		canvas.Call("setAttribute", "width", w)
		_ = b.Resize(h, w)
		return nil
	}))

	// Infinite ticker
	timer := time.NewTicker(time.Millisecond * 200)
	defer timer.Stop()

	ctx := canvas.Call("getContext", "2d")

	for {
		select {
		case <-timer.C:
			b.Step()
			b.Draw(canvas, ctx)
		}
	}
}
