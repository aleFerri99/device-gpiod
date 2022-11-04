package driver

import (
	"log"
	"net/http"
	"time"
)

var (
	connectionChannel = make(chan bool)
)

func connected() {
	for {
		_, err := http.Get("http://clients3.google.com/generate_204")
		if err != nil {
			pushConnectionStatus(false)
		}
		pushConnectionStatus(true)
		time.Sleep(30 * time.Second)
	}
}

func pushConnectionStatus(connection bool) {
	connectionChannel <- connection
}

func ConnectionCheck() {
	go connected()
	checkLoop := 0
	for {
		connAck := <-connectionChannel
		if !connAck && checkLoop == 0 {
			checkLoop = 1
			log.Println("Check connection")
			SetFlashOn('R')
			go Flashing('R')
		} else if connAck {
			checkLoop = 0
			SetFlashOff('R')
		}
	}
}
