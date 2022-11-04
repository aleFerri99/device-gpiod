package gpio

import (
	"errors"
	"log"

	"github.com/warthog618/gpiod"
)

type GPIO struct {
	Name           string `yaml:"name"`
	Chip           string `yaml:"chip"`
	Line           int    `yaml:"line"`
	State          bool
	gpioLine       *gpiod.Line
	gpioSensorLine *gpiod.Line
}

func (gpio *GPIO) Up() error {

	var err error

	err = gpio.setupOutputLine(1)
	if err != nil {
		log.Printf("Error setting up resource %d from chip %s. Error: %s", gpio.Line, gpio.Chip, err)
		return err
	}

	err = gpio.releaseLine()
	if err != nil {
		log.Printf("Error releasing resource %d from chip %s. Error: %s", gpio.Line, gpio.Chip, err)
		return err
	}

	return nil
}

func (gpio *GPIO) Down() error {

	var err error

	err = gpio.setupOutputLine(0)
	if err != nil {
		log.Printf("Error setting up resource %d from chip %s. Error: %s", gpio.Line, gpio.Chip, err)
		return err
	}

	err = gpio.releaseLine()
	if err != nil {
		log.Printf("Error releasing resource %d from chip %s. Error: %s", gpio.Line, gpio.Chip, err)
		return err
	}

	return nil
}

func (gpio *GPIO) ReadGpio() (int, error) {

	if gpio.gpioLine == nil {
		log.Printf("Resource %d of %s is not available", gpio.Line, gpio.Chip)
		return -1, errors.New("resource is not available")
	}

	value, err := gpio.gpioLine.Value()
	if err != nil {
		log.Printf("Error reading status of resource %d from chip %s. Error: %s", gpio.Line, gpio.Chip, err)
		return -1, err
	}

	err = gpio.releaseLine()
	if err != nil {
		log.Printf("Error releasing resource %d from chip %s. Error: %s", gpio.Line, gpio.Chip, err)
		return -1, err
	}

	return value, nil
}

func (gpio *GPIO) setupOutputLine(state int) error {
	var err error
	gpio.gpioLine, err = gpiod.RequestLine(gpio.Chip, gpio.Line, gpiod.AsOutput(state)) // Setup lines to default starting state
	if err != nil {
		log.Printf("Error setting up required resources. Error: %s", err)
		return err
	}
	return nil
}

func (gpio *GPIO) setupInputLine() error {
	var err error
	gpio.gpioLine, err = gpiod.RequestLine(gpio.Chip, gpio.Line, gpiod.AsInput) // Setup lines to default starting state
	if err != nil {
		log.Printf("Error setting up required resources. Error: %s", err)
		return err
	}
	return nil
}

func (gpio *GPIO) SetAsInput() error {
	return gpio.setupInputLine()
}

func (gpio *GPIO) SetAsOutput(state int) error {
	return gpio.setupOutputLine(state)
}

func (gpio *GPIO) Release() error {
	return gpio.releaseLine()
}

func (gpio *GPIO) releaseLine() error {
	return gpio.gpioLine.Close()
}
