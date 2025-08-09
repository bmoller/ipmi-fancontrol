// Package coretemp retrieves CPU core temperatures via the hwmon coretemp
// driver.
package coretemp

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/bmoller/ipmi-fancontrol/logging"
)

const (
	hwmonBase         = "/sys/class/hwmon" // where hwmon devices are found
	tempFilePatternRE = `^temp\d+_input$`  // match files for individual temps
)

var (
	devices         []string       // stores discovered hwmon coretemp devices
	tempFilePattern *regexp.Regexp // compiled regex for reuse
)

func init() {
	devices = make([]string, 0)
	tempFilePattern = regexp.MustCompile(tempFilePatternRE)
}

// discoverDevices searches the hwmon sys tree for coretemp devices and stores
// any it finds. Errors that prevent any results are returned in err, but it's
// possible to receive an error back AND have the devices updated successfully.
func discoverDevices() (err error) {
	devices = make([]string, 0)
	if entries, err := fs.ReadDir(os.DirFS(hwmonBase), "."); err != nil {
		logging.Errorln(err)
		return fmt.Errorf("failed to read directory %s", hwmonBase)
	} else {
		for _, entry := range entries {
			if name, err := os.ReadFile(filepath.Join("/sys/class", "hwmon", entry.Name(), "name")); err != nil {
				logging.Errorln(err)
				logging.Errorf("unable to read name for hwmon device %s", entry.Name())
			} else {
				if strings.TrimSpace(string(name)) == "coretemp" {
					devices = append(devices, entry.Name())
				}
			}
		}
	}

	if len(devices) == 0 {
		err = fmt.Errorf("no coretemp hwmon devices found")
	}

	return
}

// getDeviceTemps retrieves all core temperatures found for the given device.
// Any errors encountered will be returned in err, but if the function is able
// to continue working it will do so. This means that t can have valid values
// even when err is not nil.
func getDeviceTemps(device string) (t []float64, err error) {
	if entries, err := fs.ReadDir(os.DirFS(hwmonBase), device); err != nil {
		logging.Errorln(err)
		return nil, fmt.Errorf("failed to list the directory for hwmon device %s", device)
	} else {
		t = make([]float64, 0)
		for _, entry := range entries {
			if tempFilePattern.MatchString(entry.Name()) {
				value, err := os.ReadFile(filepath.Join(hwmonBase, device, entry.Name()))
				if err != nil {
					logging.Errorln(err)
					logging.Errorf("failed to read %s for hwmon device %s", entry.Name(), device)
					break
				}
				if f, err := strconv.ParseFloat(strings.TrimSpace(string(value)), 64); err != nil {
					logging.Errorln(err)
					logging.Errorf("failed to parse float value for %s for device %s", entry.Name(), device)
				} else {
					t = append(t, f/1000)
				}
			}
		}
	}

	return
}

// GetTemps queries the hwmon sysfs for current termperature readings and
// returns them in t.
//
// Any error encountered is returned in err, but the function will continue to
// query temperatures until it has checked all devices found. Known devices are
// cached to improve speed for subsequent calls.
func GetTemps() (t []float64, err error) {
	if len(devices) == 0 {
		if err = discoverDevices(); err != nil {
			logging.Errorln(err)
			return nil, fmt.Errorf("temperatures requested, but no coretemp devices found")
		}
	}

	t = make([]float64, 0)
	for _, device := range devices {
		if temps, e := getDeviceTemps(device); e != nil {
			logging.Errorln(e)
			err = fmt.Errorf("failed to retrieve temperatures for hwmon device %s", device)
		} else {
			t = append(t, temps...)
		}
	}

	return
}

// MaxTemperature queries all known coretemp devices for all of their
// temperatures and returns the single highest observed value in t. Any error
// encountered is returned in err, but t can still hold a valid value in the
// case of error.
func MaxTemperature() (t float64, err error) {
	temps, e := GetTemps()
	if e != nil {
		err = e
	}
	for _, temp := range temps {
		if temp > t {
			t = temp
		}
	}

	return
}
