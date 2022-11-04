// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018-2021 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// This package provides a simple example implementation of
// ProtocolDriver interface.
package driver

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/edgexfoundry/device-gpiod/gpio"
	"github.com/edgexfoundry/device-sdk-go/v2/pkg/interfaces"
	"github.com/edgexfoundry/device-sdk-go/v2/pkg/service"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/models"

	"github.com/edgexfoundry/device-sdk-go/v2/example/config"
	sdkModels "github.com/edgexfoundry/device-sdk-go/v2/pkg/models"
)

type SimpleDriver struct {
	lc            logger.LoggingClient
	asyncCh       chan<- *sdkModels.AsyncValues
	deviceCh      chan<- []sdkModels.DiscoveredDevice
	GpioList      *gpio.GPIOList
	Verbose       bool
	serviceConfig *config.ServiceConfig
}

type Config struct {
	PumpTimer     time.Duration
	EnableClean   bool
	CleanTimer    time.Duration
	EnableReverse bool
	ReverseTimer  time.Duration
	GravityTimer  time.Duration
	CommandGap    time.Duration
}

const (
	MAX_RETRY         = 5
	MIN_PUMP          = 5
	MIN_COMMAND_GAP   = time.Duration(10) * time.Minute
	MIN_CLEAN_TIMER   = time.Duration(5) * time.Minute
	MIN_REVERSE_TIMER = time.Duration(5) * time.Minute
	MIN_GRAVITY_TIMER = time.Duration(5) * time.Minute
	switchingTimer    = time.Duration(15) * time.Second
	openingTimer      = time.Duration(5) * time.Second
)

var (
	startTs       = flag.Int64("startTs", 0, "TimeStamp at which the pump is turned on")
	pumpTimer     = flag.Int64("pumpTimer", 0, "Time span that defines pump up status")
	enableClean   = flag.Bool("enableClean", false, "ENV flag use to select if clean circuit is available or not")
	enableReverse = flag.Bool("enableReverse", false, "ENV flag use to select if reverse circuit is available or not")
	cleanTimer    = flag.Duration("cleanTimer", time.Duration(0), "Time span that defines cleaning process")
	reverseTimer  = flag.Duration("reverseTimer", time.Duration(0), "Time span that defines reversing process")
	gravityTimer  = flag.Duration("gravityTimer", time.Duration(0), "Time span used to make the circuit remove fluids after cleaning process")
	commandGap    = flag.Duration("commandGap", time.Duration(0), "Time span between consecutive commands")
	gpioConfig    *Config
)

// Initialize performs protocol-specific initialization for the device
// service.
func (s *SimpleDriver) Initialize(lc logger.LoggingClient, asyncCh chan<- *sdkModels.AsyncValues, deviceCh chan<- []sdkModels.DiscoveredDevice) error {
	s.lc = lc
	s.asyncCh = asyncCh
	s.deviceCh = deviceCh
	s.serviceConfig = &config.ServiceConfig{}
	pumpChannel := make(chan gpio.GPIO)

	pump, err := time.ParseDuration(os.Getenv("PUMP_TIMEOUT"))
	if err != nil {
		log.Printf("Cannot parse pump timeout. Picking default value...")
		*pumpTimer = int64(time.Duration(5) * time.Minute)
	} else {
		*pumpTimer = int64(pump.Seconds())
		if *pumpTimer < MIN_PUMP*int64(time.Minute.Seconds()) {
			*pumpTimer = MIN_PUMP * int64(time.Minute.Seconds())
		}
	}

	*commandGap, err = time.ParseDuration(os.Getenv("COMMAND_GAP"))
	if err != nil {
		log.Printf("Cannot parse command gap. Picking default value...")
		*commandGap = time.Duration(60) * time.Minute
	}
	if *commandGap < MIN_COMMAND_GAP {
		*commandGap = MIN_COMMAND_GAP
	}

	*cleanTimer, err = time.ParseDuration(os.Getenv("CLEAN_TIMEOUT"))
	if err != nil {
		log.Printf("Cannot parse clean timeout. Picking default value...")
		*cleanTimer = time.Duration(5) * time.Minute
	}
	if *cleanTimer < MIN_CLEAN_TIMER {
		*cleanTimer = MIN_CLEAN_TIMER
	}

	*reverseTimer, err = time.ParseDuration(os.Getenv("REVERSE_TIMEOUT"))
	if err != nil {
		log.Printf("Cannot parse reverse timeout. Picking default value...")
		*reverseTimer = time.Duration(5) * time.Minute
	}
	if *reverseTimer < MIN_REVERSE_TIMER {
		*reverseTimer = MIN_REVERSE_TIMER
	}

	*gravityTimer, err = time.ParseDuration(os.Getenv("GRAVITY_TIMEOUT"))
	if err != nil {
		log.Printf("Cannot parse gravity timeout. Picking default value...")
		*gravityTimer = time.Duration(5) * time.Minute
	}
	if *gravityTimer < MIN_GRAVITY_TIMER {
		*gravityTimer = MIN_GRAVITY_TIMER
	}

	*enableClean, err = strconv.ParseBool(os.Getenv("ENABLE_CLEAN"))
	if err != nil {
		log.Printf("Cannot parse enable clean. Picking default value...")
		*enableClean = false
	}

	*enableReverse, err = strconv.ParseBool(os.Getenv("ENABLE_REVERSE"))
	if err != nil {
		log.Printf("Cannot parse enable reverse. Picking default value...")
		*enableReverse = false
	}

	gpioConfig = &Config{
		PumpTimer:     time.Duration(*pumpTimer),
		EnableClean:   *enableClean,
		CleanTimer:    *cleanTimer,
		EnableReverse: *enableReverse,
		ReverseTimer:  *reverseTimer,
		GravityTimer:  *gravityTimer,
		CommandGap:    *commandGap,
	}

	pumpGpio, reverseGpio, cleanGpio, openValveGpio, switchingValveGpio := -1, -1, -1, -1, -1
	var pumpChip, reverseChip, cleanChip, openValveChip, switchingValveChip string
	for _, gpio := range s.GpioList.Gpio {
		switch gpio.Name {
		case os.Getenv("START_TRIGGER"):
			pumpGpio = gpio.Line
			pumpChip = gpio.Chip
		case os.Getenv("REVERSE_TRIGGER"):
			reverseGpio = gpio.Line
			reverseChip = gpio.Chip
		case os.Getenv("CLEAN_TRIGGER"):
			cleanGpio = gpio.Line
			cleanChip = gpio.Chip
		case os.Getenv("OPEN_VALVE"):
			openValveGpio = gpio.Line
			openValveChip = gpio.Chip
		case os.Getenv("SWITCHING_VALVE"):
			switchingValveGpio = gpio.Line
			switchingValveChip = gpio.Chip
		default:
			log.Printf("Unknown gpio %s.", gpio.Name)
		}
	}

	log.Printf(`
	Device GPIO configuration:
	PUMP PIN: %d, PUMP CHIP: %s
	REVERSE PIN: %d, REVERSE CHIP: %s, REVERSE ENABLED: %t
	CLEAN PIN: %d, CLEAN CHIP: %s, CLEAN ENABLED: %t
	OPEN VALVE PIN: %d, OPNE VALVE CHIP: %s, OPEN VALVE AVAILABLE: %t
	SWITCHING VALVE PIN: %d, SWITCHING VALVE CHIP: %s, SWITCHING VALVE AVAILABLE: %t
	`, pumpGpio, pumpChip,
		reverseGpio, reverseChip, *enableReverse,
		cleanGpio, cleanChip, *enableClean,
		openValveGpio, openValveChip, *enableClean,
		switchingValveGpio, switchingValveChip, *enableClean,
	)

	ds := service.RunningService()

	if err := ds.LoadCustomConfig(s.serviceConfig, "SimpleCustom"); err != nil {
		return fmt.Errorf("unable to load 'SimpleCustom' custom configuration: %s", err.Error())
	}

	lc.Infof("Custom config is: %v", s.serviceConfig.SimpleCustom)

	if err := s.serviceConfig.SimpleCustom.Validate(); err != nil {
		return fmt.Errorf("'SimpleCustom' custom configuration validation failed: %s", err.Error())
	}

	if err := ds.ListenForCustomConfigChanges(
		&s.serviceConfig.SimpleCustom.Writable,
		"SimpleCustom/Writable", s.ProcessCustomConfigChanges); err != nil {
		return fmt.Errorf("unable to listen for changes for 'SimpleCustom.Writable' custom configuration: %s", err.Error())
	}

	s.gpioHandler(pumpChannel)

	registered := interfaces.DeviceServiceSDK.Devices(interfaces.Service())
	for _, device := range registered {
		log.Printf("Device: %v", device)
	}

	go ConnectionCheck()

	return nil
}

func (s *SimpleDriver) gpioHandler(pumpChannel chan gpio.GPIO) {
	// Handle GPIO actuation
	var pump, reversePump, clean, openValve, switchingValve, light gpio.GPIO
	for _, gpio := range s.GpioList.Gpio {
		switch name := gpio.Name; {
		case name == os.Getenv("START_TRIGGER"):
			pump = gpio
		case name == os.Getenv("REVERSE_TRIGGER"):
			if *enableReverse {
				reversePump = gpio
			}
		case name == os.Getenv("CLEAN_TRIGGER"):
			if *enableClean {
				clean = gpio
			}
		case name == os.Getenv("OPEN_VALVE"):
			if *enableClean {
				openValve = gpio
			}
		case name == os.Getenv("SWITCHING_VALVE"):
			if *enableClean {
				switchingValve = gpio
			}
		case strings.Contains(name, os.Getenv("LIGHT")):
			light = gpio
			HandleLight(light)
		default:
			log.Printf("Unknown gpio %s.", gpio.Name)
		}
	}
	// Define GPIO sequence by starting go rotutines and triggering start event
	go s.handleStartGpio(pumpChannel, reversePump, clean, openValve, switchingValve, light)
	pumpChannel <- pump
}

func (s *SimpleDriver) handleStartGpio(
	pumpChannel chan gpio.GPIO,
	reverse gpio.GPIO,
	clean gpio.GPIO,
	openValve gpio.GPIO,
	switchingValve gpio.GPIO,
	light gpio.GPIO) {
	gpio := <-pumpChannel

	// Wait for device service to be available
	// FA SCHIFO MA NON ABBIAMO ALTERNATIVA FIN QUANDO NON VIENE FIXATO L'ERRORE DEL CORE METADATA
	attempt := 0
	startPipeline := false
	for !startPipeline {
		//log.Printf("DEVICES: %v", interfaces.Service().Devices())
		//for _, device := range interfaces.Service().Devices() {
		//	err := interfaces.Service().UpdateDevice(device)
		//	if err != nil {
		//		log.Printf("Cannot update device %s in core MetaData and Cache. Error: %s", device.Name, err)
		//		time.Sleep(5 * time.Second)
		//		continue
		//	}
		//}
		ds := interfaces.Service()
		_, errGpio := ds.GetDeviceByName(ds.Name())
		if errGpio != nil {
			attempt++
			log.Printf("Attempt: %d. Device '%s' not available", attempt, ds.Name())
			if attempt > MAX_RETRY {
				os.Exit(0)
			}
			time.Sleep(5 * time.Second)
			continue
		}
		response, errModbus := http.Get(os.Getenv("MODBUS_DEVICE_ENDPOINT"))
		if errModbus != nil {
			log.Printf("Device 'Modbus-Device' not available. Error: %s", errModbus)
			time.Sleep(5 * time.Second)
			continue
		}
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Printf("Cannot fetch HTTP response body. Error: %s", err)
			log.Printf("HTTP response status code: %d", response.StatusCode)
			if response.StatusCode == 200 {
				startPipeline = true
			}
			continue
		}
		log.Printf("Modbus-Device response: %s", string(body))
		startPipeline = true
	}
	sleepForGap := false

	for {
		if !gpio.State {
			err := gpio.Up()
			if err != nil {
				err = Up('R')
				if err != nil {
					log.Printf("Error: %s", err)
				}
				log.Printf("Cannot activate pump on gpio: %d. Error: %s", gpio.Line, err)
				time.Sleep(time.Second)
				continue
			}
			gpio.State = true
			// Get timestamp to temporize GPIO flow control
			*startTs = time.Now().Unix()
			err = Up('G')
			if err != nil {
				log.Printf("Error: %s", err)
			}
			// Handle async core data communication
			s.handleAsyncCommunication(gpio)
		} else {
			if time.Now().Unix()-*startTs >= *pumpTimer {
				err := gpio.Down()
				if err != nil {
					err = Up('R')
					if err != nil {
						log.Printf("Error: %s", err)
					}
					log.Printf("Cannot deactivate pump on gpio: %d. Error: %s", gpio.Line, err)
					time.Sleep(time.Second)
					continue
				}
				gpio.State = false
				err = Down('G')
				if err != nil {
					log.Printf("Error: %s", err)
				}
				// Add logic to handle pump reverse and electrovalves actuation
				if *enableReverse {
					s.handleReverseGpio(reverse, clean, openValve, switchingValve, light)
				}
				sleepForGap = true
				// Handle async core data communication
				s.handleAsyncCommunication(gpio)
			} else {
				log.Printf("Pump will run for %d s...", *pumpTimer-(time.Now().Unix()-*startTs))
				time.Sleep(time.Duration(*pumpTimer) * time.Second)
			}
		}
		// Sleep for the specified commandGap time...
		if sleepForGap {
			// Wait for commandGap timeout
			log.Printf("Pump timeout. Sleeping for %d minutes...", int64(commandGap.Minutes()))
			time.Sleep(*commandGap)
			sleepForGap = false
		}
	}
}

func (s *SimpleDriver) handleReverseGpio(
	reverse gpio.GPIO,
	clean gpio.GPIO,
	openValve gpio.GPIO,
	switchingValve gpio.GPIO,
	light gpio.GPIO) {
	log.Println("Reverting pump...")
	err := reverse.Up()
	if err != nil {
		err = Up('R')
		if err != nil {
			log.Printf("Error: %s", err)
		}
		log.Printf("Cannot start cleaning process on gpio: %d. Error: %s", reverse.Line, err)
		return
	}
	reverse.State = true
	SetFlashOn('G')
	go Flashing('G')
	// Handle async core data communication
	s.handleAsyncCommunication(reverse)
	// Sleep for user defined cleaning duration
	time.Sleep(*reverseTimer)
	// Toggle Reverse pump GPIO
	reverse.Down()
	if err != nil {
		err = Up('R')
		if err != nil {
			log.Printf("Error: %s", err)
		}
		log.Printf("Cannot stop reverting process on gpio: %d. Error: %s", reverse.Line, err)
		return
	}
	reverse.State = false
	SetFlashOff('G')
	// Handle async core data communication
	s.handleAsyncCommunication(reverse)
	log.Println("Circuit is now empty!")
	// Launch Cleaning process
	if *enableClean {
		s.handleCleanGpio(clean, openValve, switchingValve, light)
	}
}

func (s *SimpleDriver) handleCleanGpio(
	clean gpio.GPIO,
	openValve gpio.GPIO,
	switchingValve gpio.GPIO,
	light gpio.GPIO) {
	log.Printf("Step 1 -> Switching hydraulic circuit with switching valve on gpio %d", switchingValve.Line)
	err := switchingValve.Up()
	if err != nil {
		err = Up('R')
		if err != nil {
			log.Printf("Error: %s", err)
		}
		log.Printf("Cannot switch the hydraulic circuit. Error: %s", err)
		return
	}
	time.Sleep(switchingTimer)
	log.Printf("Step 2 -> Enable cleaning inlet with open valve on gpio %d", openValve.Line)
	err = openValve.Up()
	if err != nil {
		err = Up('R')
		if err != nil {
			log.Printf("Error: %s", err)
		}
		log.Printf("Cannot open the washing circuit. Error: %s", err)
		return
	}
	time.Sleep(openingTimer)
	log.Println("Step 3 -> Performing circuit clean up...")
	err = clean.Up()
	if err != nil {
		err = Up('R')
		if err != nil {
			log.Printf("Error: %s", err)
		}
		log.Printf("Cannot start cleaning process on gpio: %d. Error: %s", clean.Line, err)
		return
	}
	clean.State = true
	err = Up('Y')
	if err != nil {
		log.Printf("Error: %s", err)
	}
	// Handle async core data communication
	s.handleAsyncCommunication(clean)
	// Sleep for user defined cleaning duration
	time.Sleep(*cleanTimer)
	// Toggle Clean pump GPIO
	clean.Down()
	if err != nil {
		err = Up('R')
		if err != nil {
			log.Printf("Error: %s", err)
		}
		log.Printf("Cannot stop cleaning process on gpio: %d. Error: %s", clean.Line, err)
		return
	}
	clean.State = false
	err = Down('Y')
	if err != nil {
		log.Printf("Error: %s", err)
	}
	// Handle async core data communication
	s.handleAsyncCommunication(clean)
	log.Printf("Restoring circuit behaviour...")
	err = openValve.Down()
	if err != nil {
		err = Up('R')
		if err != nil {
			log.Printf("Error: %s", err)
		}
		log.Printf("Cannot close the washing circuit. Error: %s", err)
		return
	}
	time.Sleep(openingTimer)
	// Add some delay to make cleaning liquid exit by gravity
	time.Sleep(*gravityTimer)
	err = switchingValve.Down()
	if err != nil {
		err = Up('R')
		if err != nil {
			log.Printf("Error: %s", err)
		}
		log.Printf("Cannot restore hydraulic circuit behaviour. Error: %s", err)
		return
	}
	time.Sleep(switchingTimer)
	log.Println("Circuit cleaned!")
}

func (s *SimpleDriver) handleAsyncCommunication(gpio gpio.GPIO) {
	res := make([]*sdkModels.CommandValue, 1)
	gpiod, err := json.Marshal(map[string]interface{}{
		"gpio":       gpio,
		"gpioConfig": &gpioConfig,
	})
	var cv *sdkModels.CommandValue

	if err != nil {
		log.Printf("Cannot parse gpiod data to JSON. Error: %s", err)
		cv, _ = sdkModels.NewCommandValue("GPIO", common.ValueTypeString, err)
	} else {
		cv, _ = sdkModels.NewCommandValue("GPIO", common.ValueTypeString, string(gpiod))
	}
	log.Println("Pushing gpio to EdgeX Core Data")
	res[0] = cv
	asyncValues := &sdkModels.AsyncValues{
		DeviceName:    "device-gpiod",
		CommandValues: res,
	}
	s.asyncCh <- asyncValues
	s.lc.Info(fmt.Sprintf("Data sent to core data: %s", string(gpiod)))
}

// ProcessCustomConfigChanges ...
func (s *SimpleDriver) ProcessCustomConfigChanges(rawWritableConfig interface{}) {
	updated, ok := rawWritableConfig.(*config.SimpleWritable)
	if !ok {
		s.lc.Error("unable to process custom config updates: Can not cast raw config to type 'SimpleWritable'")
		return
	}

	s.lc.Info("Received configuration updates for 'SimpleCustom.Writable' section")

	previous := s.serviceConfig.SimpleCustom.Writable
	s.serviceConfig.SimpleCustom.Writable = *updated

	if reflect.DeepEqual(previous, *updated) {
		s.lc.Info("No changes detected")
		return
	}

	// Now check to determine what changed.
	// In this example we only have the one writable setting,
	// so the check is not really need but left here as an example.
	// Since this setting is pulled from configuration each time it is need, no extra processing is required.
	// This may not be true for all settings, such as external host connection info, which
	// may require re-establishing the connection to the external host for example.
	if previous.DiscoverSleepDurationSecs != updated.DiscoverSleepDurationSecs {
		s.lc.Infof("DiscoverSleepDurationSecs changed to: %d", updated.DiscoverSleepDurationSecs)
	}
}

// HandleReadCommands triggers a protocol Read operation for the specified device.
func (s *SimpleDriver) HandleReadCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []sdkModels.CommandRequest) (res []*sdkModels.CommandValue, err error) {
	s.lc.Debugf("SimpleDriver.HandleReadCommands: protocols: %v resource: %v attributes: %v", protocols, reqs[0].DeviceResourceName, reqs[0].Attributes)

	return nil, fmt.Errorf("RestDriver.HandleReadCommands; read commands not supported")
}

// HandleWriteCommands passes a slice of CommandRequest struct each representing
// a ResourceOperation for a specific device resource.
// Since the commands are actuation commands, params provide parameters for the individual
// command.
func (s *SimpleDriver) HandleWriteCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []sdkModels.CommandRequest,
	params []*sdkModels.CommandValue) error {

	return fmt.Errorf("RestDriver.HandleWriteCommands; write commands not supported")
}

// Stop the protocol-specific DS code to shutdown gracefully, or
// if the force parameter is 'true', immediately. The driver is responsible
// for closing any in-use channels, including the channel used to send async
// readings (if supported).
func (s *SimpleDriver) Stop(force bool) error {
	// Then Logging Client might not be initialized
	if s.lc != nil {
		s.lc.Debugf("SimpleDriver.Stop called: force=%v", force)
	}
	return nil
}

// AddDevice is a callback function that is invoked
// when a new Device associated with this Device Service is added
func (s *SimpleDriver) AddDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	s.lc.Debugf("a new Device is added: %s", deviceName)
	return nil
}

// UpdateDevice is a callback function that is invoked
// when a Device associated with this Device Service is updated
func (s *SimpleDriver) UpdateDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	s.lc.Debugf("Device %s is updated", deviceName)
	return nil
}

// RemoveDevice is a callback function that is invoked
// when a Device associated with this Device Service is removed
func (s *SimpleDriver) RemoveDevice(deviceName string, protocols map[string]models.ProtocolProperties) error {
	s.lc.Debugf("Device %s is removed", deviceName)
	return nil
}

// Discover triggers protocol specific device discovery, which is an asynchronous operation.
// Devices found as part of this discovery operation are written to the channel devices.
func (s *SimpleDriver) Discover() {
	proto := make(map[string]models.ProtocolProperties)
	proto["other"] = map[string]string{"Address": "simple02", "Port": "301"}

	device2 := sdkModels.DiscoveredDevice{
		Name:        "Simple-Device02",
		Protocols:   proto,
		Description: "found by discovery",
		Labels:      []string{"auto-discovery"},
	}

	proto = make(map[string]models.ProtocolProperties)
	proto["other"] = map[string]string{"Address": "simple03", "Port": "399"}

	device3 := sdkModels.DiscoveredDevice{
		Name:        "Simple-Device03",
		Protocols:   proto,
		Description: "found by discovery",
		Labels:      []string{"auto-discovery"},
	}

	res := []sdkModels.DiscoveredDevice{device2, device3}

	time.Sleep(time.Duration(s.serviceConfig.SimpleCustom.Writable.DiscoverSleepDurationSecs) * time.Second)
	s.deviceCh <- res
}
