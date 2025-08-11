// The cmd package provides the main user interface. It parses configuration
// variables and contains the main execution loop.
package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/bmoller/ipmi-fancontrol/bmc"
	"github.com/bmoller/ipmi-fancontrol/coretemp"
	"github.com/bmoller/ipmi-fancontrol/logging"
)

const (
	idleTemp       = 40      // expected temperature when the server is idle
	intervalString = "3s"    // how long to sleep between readings/updates
	loadTemp       = 90      // temperature at which fans go to 100% power
	minFanSpeed    = 25      // floor value for fan power setting
	semVer         = "1.0.0" // version string
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
	interval time.Duration // holds the parsed string interval above
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
	// handle flags
	if len(os.Args) > 1 {
		printHelp, exitVal := false, 0
		for _, flag := range os.Args[1:] {
			switch strings.ToLower(flag) {
			case "-h", "--help":
				printHelp = true
			case "-v", "--version":
				fmt.Printf(" %s built on %s\n", semVer, runtime.Version())
			default:
				fmt.Printf("unrecognized flag '%s'\n", flag)
				printHelp = true
				exitVal = 1
			}
		}
		if printHelp {
			fmt.Println(usage)
		}
		os.Exit(exitVal)
	}

	setLogLevel()
	logging.Debugln("entering Start")
	defer logging.Debugln("exiting Start")

	if err := setFanIDs(); err != nil {
		logging.Errorln(err)
		logging.Fatalln("failed to detect and load sensors")
	}

	if err := bmc.EnableManualControl(); err != nil {
		logging.Errorln(err)
		logging.Fatalln("failed to enable manual fan control")
	}
	// make sure that we return control to the BMC on exit
	defer bmc.EnableAutoCurve()

	// main loop
	for {
		if t, err := coretemp.MaxTemperature(); err != nil {
			logging.Errorln(err)
			logging.Errorln("setting fail-safe fan speed of 75%")
			if err := bmc.SetAllFans(75); err != nil {
				logging.Errorln(err)
				logging.Fatalln("failed to set fail-safe speed; exiting")
			}
		} else {
			logging.Debugf("max core temperature read is %f", t)
			desiredSpeed := (int(t)-idleTemp)*(100-minFanSpeed)/(loadTemp-idleTemp) + minFanSpeed
			if desiredSpeed < minFanSpeed {
				desiredSpeed = minFanSpeed
			}
			logging.Debugf("target fan power is %d%%", desiredSpeed)
			if err := bmc.SetAllFans(uint8(desiredSpeed)); err != nil {
				logging.Errorln(err)
			}
		}

		logging.Debugf("sleeping for %d seconds", interval)
		time.Sleep(interval)
	}
}
