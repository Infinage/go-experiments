//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"
)

func main() {
	alerter := js.FuncOf(func(this js.Value, args []js.Value) any {
		var repr strings.Builder
		for idx, arg := range args {
			if idx > 0 {
				repr.WriteRune(' ')
			}
			repr.WriteString(arg.String())
		}

		js.Global().Call("alert", "hello: " + repr.String())
		return nil
	})

	js.Global().Call("alert", "Hello WASM!")
	js.Global().Set("custom_alert", alerter)
	select {}
}
