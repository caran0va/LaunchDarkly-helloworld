package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	ld "github.com/launchdarkly/go-server-sdk/v6"
)

// Set sdkKey to your LaunchDarkly SDK key.
var sdkKey = ""
var flagChan = make(chan bool)
var myFlag = false
var lastFlag = !myFlag

// Set featureFlagKey to the feature flag key you want to evaluate.
const featureFlagKey = "my-flag"

func main() {

	wg := sync.WaitGroup{}

	//load .env (contains sdkKey)
	if err := godotenv.Load(".env"); err != nil {
		log.Fatal("Error loading .env file")
	}

	sdkKey = os.Getenv("LD_SDKKEY")
	if sdkKey == "" {
		log.Fatal("Please edit .env to set sdkKey to your LaunchDarkly SDK key first")
	}

	client, _ := ld.MakeClient(sdkKey, 5*time.Second)
	if client.Initialized() {
		log.Printf("SDK successfully initialized!")
	} else {
		log.Fatal("SDK failed to initialize")
	}
	wg.Add(1)
	SetupCloseHandler(client, &wg)
	// Set up the evaluation context. This context should appear on your LaunchDarkly contexts dashboard
	// soon after you run the demo.
	context := ldcontext.NewBuilder("helloworld-context-key").Name("Cara").Build()

	//periodically check LD for flag update
	go checkFlag(context, client)
	//listens for changes in internal flag variable changes, and notifies the log to let us know featureflag change
	go listenToFlag()

	wg.Wait()

}

func listenToFlag() {
	for {
		myFlag = <-flagChan
		if myFlag != lastFlag {
			//when the flag channel changes, let us know
			log.Printf("Feature flag '%s' is %t for this context", featureFlagKey, myFlag)
			lastFlag = myFlag
		}

	}
}

func checkFlag(context ldcontext.Context, client *ld.LDClient) {
	for {
		time.Sleep(time.Second)                                                // dont want to be sending this request constantly, 1 second delay is enough for LD to keep up though it seems.
		flagValue, err := client.BoolVariation(featureFlagKey, context, false) // checks LD for boolean flag value
		if err != nil {
			log.Printf("error: " + err.Error())
		}

		//write new value to the channel
		flagChan <- flagValue
	}
}

// sets up a handler that waits forSIGTERM or os.interrupt (control-C) and gracefully closes LDClient
func SetupCloseHandler(client *ld.LDClient, wg *sync.WaitGroup) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C pressed in Terminal")
		wg.Done()
		onClose(client)
		os.Exit(0)
	}()
}

func onClose(client *ld.LDClient) {
	// Here we ensure that the SDK shuts down cleanly and has a chance to deliver analytics
	// events to LaunchDarkly before the program exits. If analytics events are not delivered,
	// the context attributes and flag usage statistics will not appear on your dashboard. In
	// a normal long-running application, the SDK would continue running and events would be
	// delivered automatically in the background.
	client.Close()
}
