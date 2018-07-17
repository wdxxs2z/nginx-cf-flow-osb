package main

import (
	"github.com/cloudfoundry-community/go-cfclient"
	"fmt"
	"log"
	"time"
)

func getAppState(client *cfclient.Client, appGuid string, errChan chan error, success chan bool){
	appStates, err := client.GetAppStats(appGuid)
	if err != nil{
		errChan <- err
	}
	s := appStates["0"].State
	switch s {
	case "RUNNING":
		success <- true
	case "STARTING":
		getAppState(client, appGuid, errChan, success)
		fmt.Println("starting...")
	case "DOWN":
		errChan <- fmt.Errorf("app down.")
	}
}

func main() {
	errChan := make(chan error)
	successChan := make(chan  bool)
	config := &cfclient.Config{
		ApiAddress:        "https://api.local.pcfdev.io",
		Username:          "admin",
		Password:          "admin",
		SkipSslValidation: true,
	}
	c, err := cfclient.NewClient(config)
	if err != nil {
		log.Fatal(err)
	}
	go getAppState(c, "fb7e12e9-6549-404c-9885-23b5b6df17c7", errChan, successChan)
	select {
	case <- errChan :
		fmt.Println("err....")
	case <- successChan :
		fmt.Println("success...")
	case <- time.After(time.Duration(7 * time.Second)):
		fmt.Println("no data respose.")
	}

	close(successChan)
	close(errChan)
}
