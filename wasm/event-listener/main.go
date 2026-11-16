//go:build wasm && js

package main

import (
	"fmt"
	"os"
	"syscall/js"
)

func main() {
	listener := js.FuncOf(func(this js.Value, args []js.Value) any {
		kc := args[0].Get("keyCode")
		if !kc.Truthy() || kc.Type() != js.TypeNumber {
			return fmt.Errorf("Keycode invalid")
		}

		js.Global().Get("console").Call("log", fmt.Sprint("pressed: ", kc.Int()))
		return nil
	})

	doc := js.Global().Get("document")
	body := doc.Call("querySelector", "body")
	if !body.Truthy() {
		fmt.Println("body tag not found")
		os.Exit(1)
	}

	body.Call("addEventListener", "keyup", listener)
	select {}
}
