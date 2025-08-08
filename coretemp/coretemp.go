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
	hwmonBase         = "/sys/class/hwmon"
	tempFilePatternRE = `^temp\d+_input$`
)

var (
	devices         []string
	tempFilePattern *regexp.Regexp
)

func init() {
	devices = make([]string, 0)
	tempFilePattern = regexp.MustCompile(tempFilePatternRE)
}

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
