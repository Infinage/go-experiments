package main

import (
	"machine"
)

func main() {
	btnIn := machine.D11
	btnIn.Configure(machine.PinConfig{Mode: machine.PinInputPullup})

	ledOut := machine.D10
	ledOut.Configure(machine.PinConfig{Mode: machine.PinOutput})

	for {
		// With PinInputPullup, D11 is always high
		// When switch is pressed it goes low
		if !btnIn.Get() {
			ledOut.High()
			continue
		}
		ledOut.Low()
	}
}
