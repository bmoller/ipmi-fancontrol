package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/bmoller/ipmi-fancontrol/bmc"
	"github.com/bmoller/ipmi-fancontrol/logging"
)

const (
	fanIDRegex = `^0x[0-9A-Fa-f]{2}$`
)

// setLogLevel checks the environment variables for a configured level.
//
// If no variable is found, or the value isn't recognized, then the default
// level of WARN is set.
func setLogLevel() {
	v, _ := os.LookupEnv("LOG_LEVEL")
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "debug":
		logging.SetLevel(logging.DEBUG)
	case "info":
		logging.SetLevel(logging.INFO)
	case "warn", "":
		logging.SetLevel(logging.WARN)
	case "error":
		logging.SetLevel(logging.ERROR)
	case "critical":
		logging.SetLevel(logging.FATAL)
	default:
		fmt.Printf("unrecognized log level `%s`, defaulting to log level `warn`\n", v)
		logging.SetLevel(logging.WARN)
	}

	logging.Debugln("exiting setLogLevel")
}

// setFanIDs initializes the BMC interface with IDs of fan sensors.
//
// If the SENSOR_IDS environment variable is defined, this function attempts to
// load those sensors. Should this fail, or should the envvar not be defined,
// it falls back on auto-detection. If auto-detection is used and generates an
// error it is returned in err.
func setFanIDs() (err error) {
	logging.Debugln("entering setFanIDs")
	defer logging.Debugln("exiting setFanIDs")

	envValue, found := os.LookupEnv("SENSOR_IDS")
	if !found {
		logging.Infoln("SENSOR_IDS not found in environment; using fan auto-detection")
	} else if ids, err := parseFanIDs(envValue); err == nil {
		logging.Debugln("attempting to use the following sensor IDS:")
		logging.Debugln(ids)
		return bmc.LoadSensors(ids...)
	} else {
		logging.Errorln(err)
		logging.Warnln("falling back on fan auto-detection")
	}

	return bmc.DiscoverFanSensorIDs()
}

// parseFanIDs takes a environment variable's value and attempts to extract
// hexadecimal IDs.
//
// Valid values use common hex syntax, namely matching the regex
// `0x[0-9A-Fa-f]`. Any values that don't match are simply ignored and
// execution continues. The only possible error is a failure to compile the
// regex, returned in err.
func parseFanIDs(value string) (ids []uint8, err error) {
	logging.Debugln("entering parseFanIDs")
	defer logging.Debugln("exiting parseFanIDs")

	vals := strings.Split(value, ",")
	ids = make([]uint8, 0)
	re, err := regexp.Compile(fanIDRegex)
	if err != nil {
		logging.Errorln(err)
		return nil, fmt.Errorf("failed to compile fan ID regex; this should not happen")
	}
	for _, s := range vals {
		s = strings.TrimSpace(s)
		if re.Match([]byte(s)) {
			logging.Debugf("adding fan sensor ID %s", s)
			vals = append(vals, strings.ToUpper(s))
		} else {
			logging.Errorf("failed to parse fan ID %s; ignoring", s)
		}
	}

	return
}
