package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bmoller/ipmi-fancontrol/bmc"
	"github.com/bmoller/ipmi-fancontrol/coretemp"
	"github.com/bmoller/ipmi-fancontrol/logging"
)

const (
	idleTemp       = 40
	intervalString = "3s"
	loadTemp       = 90
	minFanSpeed    = 25
	usage          = `ipmi-fancontrol is a simple daemon to monitor CPU temperatures and adjust fan
speeds in response via IPMI. It is intended to be run as a foreground daemon
managed by systemd.

There are no command-line arguments, but several environment variables
affect behavior and execution. These include:

LOG_LEVEL: [ debug | info | warn | error | critical ]
           Sets the minimum level of log messages emitted.

SENSOR_IDS: If the fan sensor IDs are already known, these can be passed
            as a comma-separated list of hexadecimal ints. ipmi-fanctronol will
            then attempt to monitor these fan speeds.`
)

var (
	interval time.Duration
)

func init() {
	var e error
	if interval, e = time.ParseDuration(intervalString); e != nil {
		panic(e)
	}
}

// Start is the main entrypoint for the program.
//
// It handles parsing flags and env variables, then calls the main loop for
// execution.
func Start() {
	// handle help flag
	if len(os.Args) > 1 && (strings.ToLower(os.Args[1]) == "-h" || strings.ToLower(os.Args[1]) == "--help") {
		fmt.Println(usage)
		os.Exit(0)
	}

	setLogLevel()

	if err := setFanIDs(); err != nil {
		logging.Errorln(err)
		logging.Fatalln("failed to detect and load sensors")
	}

	if err := bmc.EnableManualControl(); err != nil {
		logging.Errorln(err)
		logging.Fatalln("failed to enable manual fan control")
	}
	defer bmc.EnableAutoCurve()

	for {
		if t, err := coretemp.MaxTemperature(); err != nil {
			logging.Errorln(err)
			logging.Errorln("setting fail-safe fan speed of 75%")
			if err := bmc.SetAllFans(75); err != nil {
				logging.Errorln(err)
				logging.Fatalln("failed to set fail-safe speed; exiting")
			}
		} else {
			desiredSpeed := (int(t)-idleTemp)*(100-minFanSpeed)/(loadTemp-idleTemp) + minFanSpeed
			if desiredSpeed < minFanSpeed {
				desiredSpeed = minFanSpeed
			}
			if err := bmc.SetAllFans(uint8(desiredSpeed)); err != nil {
				logging.Errorln(err)
			}
		}

		time.Sleep(interval)
	}
}
