/*
Normal flow:
- Traffic green for 5s, red for 2.5s
- Pedestrian red for 5s, green for 2.5s

On button press:
- Traffic goes red, Pedestrian goes green (stays until button release)
- Stays this way for 2s more before returning to normal flow
*/
package main

import (
	"machine"
	"time"
)

type SystemState int

const (
	VehicleRed   = machine.D7
	VehicleGreen = machine.D6

	pedestrianRed   = machine.D11
	pedestrianGreen = machine.D10

	btnInp = machine.D12

	StateVehicleTurn SystemState = iota
	StatePedestrianTurn
	StateInterupt
)

func init() {
	outConf := machine.PinConfig{Mode: machine.PinOutput}
	VehicleRed.Configure(outConf)
	VehicleGreen.Configure(outConf)
	pedestrianGreen.Configure(outConf)
	pedestrianRed.Configure(outConf)
	btnInp.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
}

func setLights(state SystemState) {
	switch state {
	case StateVehicleTurn:
		VehicleGreen.High()
		VehicleRed.Low()
		pedestrianGreen.Low()
		pedestrianRed.High()

	case StatePedestrianTurn, StateInterupt:
		VehicleGreen.Low()
		VehicleRed.High()
		pedestrianGreen.High()
		pedestrianRed.Low()
	}
}

func main() {
	state := StateVehicleTurn
	stateStartTime := time.Now()

	for {
		now := time.Now()
		elapsed := time.Since(stateStartTime)

		// Hijack the flow
		if !btnInp.Get() {
			state = StateInterupt
			stateStartTime = now
		}

		// If above block had executed this is effectively an no-op
		switch state {
		case StateVehicleTurn:
			if elapsed >= time.Second*5 {
				state = StatePedestrianTurn
				stateStartTime = now
			}

		case StatePedestrianTurn:
			if elapsed >= time.Millisecond*2500 {
				state = StateVehicleTurn
				stateStartTime = now
			}

		case StateInterupt:
			if elapsed >= time.Second*2 {
				state = StateVehicleTurn
				stateStartTime = now
			}
		}

		// set the appropriate light based on state
		setLights(state)
		time.Sleep(10 * time.Millisecond)
	}
}
