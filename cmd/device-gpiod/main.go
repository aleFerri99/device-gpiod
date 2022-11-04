// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
// Copyright (C) 2018-2019 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// This package provides a simple example of a device service.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"strconv"

	"github.com/edgexfoundry/device-gpiod"
	"github.com/edgexfoundry/device-gpiod/driver"
	"github.com/edgexfoundry/device-gpiod/gpio"
	"github.com/edgexfoundry/device-sdk-go/v2/pkg/startup"
)

const (
	serviceName string = "device-gpiod"
)

var (
	verbose = flag.Bool("verbose", false, "Add/Remove debug logs")
	confdir = flag.String("confdir", "", "Path to EdgeX DS configuration files")
	err     error
)

func main() {

	// Get env vars
	*verbose, err = strconv.ParseBool(os.Getenv("VERBOSE"))
	if err != nil {
		log.Printf("Cannot parse %s to bool. Taking default value -> false...", os.Getenv("VERBOSE"))
		*verbose = false
	}

	sd := driver.SimpleDriver{}
	sd.Verbose = *verbose
	sd.GpioList = &gpio.GPIOList{}

	err = sd.GpioList.Parse(os.Getenv("GPIO_CONFIG_FILE"), *verbose)
	if err != nil {
		log.Printf("Error parsing GPIO configuration. Error: %s", err)
	}

	if *verbose {
		prettyprint, err := json.MarshalIndent(&sd.GpioList, "", "\t")
		if err != nil {
			log.Printf("Failed to pretty print modbus configration file. Error: %s", err)
			prettyprint = []byte("ERROR")
		}
		log.Printf("Pretty print MODBUS configuration file:\n%s", string(prettyprint))
	}

	startup.Bootstrap(serviceName, device.Version, &sd)
}
