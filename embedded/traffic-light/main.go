package main

import (
	"machine"
	"time"
)

func main() {
	outConfig := machine.PinConfig{Mode: machine.PinOutput}

	red := machine.D10
	red.Configure(outConfig)

	green := machine.D8
	green.Configure(outConfig)

	blue := machine.D6
	blue.Configure(outConfig)

	sleepSec := 500 * time.Millisecond
	for {
		red.High()
		time.Sleep(sleepSec)
		red.Low()

		green.High()
		time.Sleep(sleepSec)
		green.Low()

		blue.High()
		time.Sleep(sleepSec)
		blue.Low()
	}
}
