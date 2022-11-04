package driver

import (
	"fmt"
	"log"
	"time"

	"github.com/edgexfoundry/device-gpiod/gpio"
)

type lights struct {
	flashing bool
	status   bool
	color    rune
	gpio     gpio.GPIO
}

var (
	green, yellow, red *lights
)

func HandleLight(g gpio.GPIO) {
	switch g.Line {
	case 5:
		green.color = 'G'
		green.gpio = g
	case 6:
		yellow.color = 'Y'
		yellow.gpio = g
	case 7:
		red.color = 'R'
		red.gpio = g
	default:
		log.Printf("Unknown light %d", g.Line)
	}
}

func Up(color rune) error {
	var err error
	switch color {
	case 'G':
		err = green.gpio.Up()
		green.status = true
		yellow.status = false
		red.status = false
	case 'Y':
		err = yellow.gpio.Up()
		yellow.status = true
		red.status = false
		green.status = false
	case 'R':
		err = red.gpio.Up()
		red.status = true
		green.status = false
		yellow.status = false
	default:
		log.Printf("Unknown color %c", color)
	}
	return err
}

func Down(color rune) error {
	var err error
	switch color {
	case 'G':
		err = green.gpio.Down()
		green.status = false
	case 'Y':
		err = yellow.gpio.Down()
		yellow.status = false
	case 'R':
		err = red.gpio.Down()
		red.status = false
	default:
		log.Printf("Unknown color %c", color)
	}
	return err
}

func Flashing(color rune) error {
	var err error
	var flashingLight *lights

	switch color {
	case 'G':
		flashingLight = green
	case 'Y':
		flashingLight = yellow
	case 'R':
		flashingLight = red
	default:
		log.Printf("Unknown color %c", color)
		return fmt.Errorf("unknown color %c", color)
	}
	for flashingLight.flashing {
		err = Up(color)
		if err != nil {
			log.Printf("Cannot start light %c. Error: %s", color, err)
			return err
		}
		time.Sleep(3 * time.Second)
		err = Down(color)
		if err != nil {
			log.Printf("Cannot stop light %c. Error: %s", color, err)
			return err
		}
	}
	return nil
}

func SetFlashOn(color rune) {
	switch color {
	case 'G':
		green.flashing = true
		yellow.flashing = false
		red.flashing = false
	case 'Y':
		yellow.flashing = true
		green.flashing = false
		red.flashing = false
	case 'R':
		red.flashing = true
		green.flashing = false
		yellow.flashing = false
	default:
		log.Printf("Unknown color %c", color)
	}
}

func SetFlashOff(color rune) {
	switch color {
	case 'G':
		green.flashing = false
	case 'Y':
		yellow.flashing = false
	case 'R':
		red.flashing = false
	default:
		log.Printf("Unknown color %c", color)
	}
}
