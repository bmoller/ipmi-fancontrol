package bmc

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/bougou/go-ipmi"

	"github.com/bmoller/ipmi-fancontrol/logging"
)

const (
	fanControl      uint8 = 0x30            // "command" byte for controlling fans
	fanSensorNameRE       = `^FAN \d+ RPM$` // regex to filter fan sensors
	fiveSeconds           = "5s"            // duration string for connection retries
)

type sensorData struct {
	Name string // human-readable name of the sensor
	M    int16
	B    int16
	K1   int8
	K2   int8
}

var (
	client        *ipmi.Client                               // persistent client to the BMC
	ctx           context.Context                            // default context passed to IPMI functions
	autoBytes     []uint8              = []uint8{0x01, 0x01} // data to enable automatic fan control
	fanData       map[uint8]sensorData                       // store sensor names and factors
	fanSensorName *regexp.Regexp                             // compiled to match actual fan sensor names
	manualBytes   []uint8              = []uint8{0x01, 0x00} // data to enable manual fan control
	retryInterval time.Duration                              // length of time between connection attempts
	setSpeedData  uint8                = 0x02                // data to set speed for all BMC fans
)

func init() {
	var err error
	if retryInterval, err = time.ParseDuration(fiveSeconds); err != nil {
		panic("failed to parse interval as a duration")
	}
	for client, err = ipmi.NewOpenClient(); err != nil; client, err = ipmi.NewOpenClient() {
		logging.Errorln("failed to to create ipmi client:")
		logging.Errorln(err)
		time.Sleep(retryInterval)
	}
	fanSensorName = regexp.MustCompile(fanSensorNameRE)
	ctx = context.Background()
	fanData = make(map[uint8]sensorData)
}

// DiscoverFanSensorIDs searches the SDR for fan sensors and stores their names
// and data.
//
// Any currently stored sensor data is cleared when this function executes. If
// querying the BMC generates an error it is returned in err.
func DiscoverFanSensorIDs() (err error) {
	clear(fanData)
	openConnection()
	defer client.Close(ctx)

	if sensors, err := client.GetSensors(ctx, ipmi.SensorFilterOptionIsSensorType(ipmi.SensorTypeFan)); err != nil {
		logging.Errorln("failed to query BMC for fan sensors")
		return err
	} else {
		for _, s := range sensors {
			// not all IPMI-reported sensors are actually fans; check the name
			if !fanSensorName.Match([]byte(s.Name)) {
				continue
			}
			fanData[s.Number] = sensorData{
				Name: s.Name,
				M:    s.Threshold.M,
				B:    s.Threshold.B,
				K1:   s.Threshold.B_Exp,
				K2:   s.Threshold.R_Exp,
			}
			logging.Debugln("added fan:")
			logging.Debugln(fanData[s.Number])
		}
	}

	return
}

// LoadSensors attempts to lookup and store all of the indicated sensors in
// ids.
//
// If no IDs are provided, or any sensor ID fails to load, automatic detection
// is used instead to find all valid fan sensors. If auto-detection is used and
// generates an error, it is returned in err.
func LoadSensors(ids ...uint8) (err error) {
	logging.Debugln("entering LoadSensors")
	defer logging.Debugln("exiting LoadSensors")

	clear(fanData)
	if len(ids) == 0 {
		logging.Debugln("no sensor IDs specified; discovering")
		return DiscoverFanSensorIDs()
	}

	openConnection()
	defer client.Close(ctx)
	for _, id := range ids {
		sensor, err := client.GetSensorByID(ctx, id)
		if err != nil {
			logging.Errorf("failed to load sensor with ID %d", id)
			logging.Errorln(err)
			logging.Warnln("falling back to automatic sensor detection")
			return DiscoverFanSensorIDs()
		}
		fanData[id] = sensorData{
			Name: sensor.Name,
			M:    sensor.Threshold.M,
			B:    sensor.Threshold.B,
			K1:   sensor.Threshold.B_Exp,
			K2:   sensor.Threshold.R_Exp,
		}
	}

	return
}

// openConnection establishes an IPMI session with the BMC.
//
// Should the connection fail it will retry at regular intervals until it
// succeeds; this is a blocking operation.
func openConnection() {
	logging.Debugln("entering openConnection")
	defer logging.Debugln("exiting openConnection")

	for err := client.Connect(ctx); err != nil; err = client.Connect(ctx) {
		logging.Errorln("failed to connect to IPMI socket")
		logging.Errorln(err)
		time.Sleep(retryInterval)
	}
}

// GetFanSpeeds queries current readings for all known fan sensors.
//
// If no sensors are currently known, it attempts automatic discovery of fan
// sensors. Should this auto-discovery fail, the error is returned in err. If
// any single sensor's reading cannot be retrieved, an error is logged, but
// execution otherwise continues with any remaining sensors.
func GetFanSpeeds() (values map[string]int16, err error) {
	logging.Debugln("entering GetFanSpeeds")
	defer logging.Debugln("exiting GetFanSpeeds")

	openConnection()
	defer client.Close(ctx)

	// check that there's actually something to do
	if len(fanData) == 0 {
		logging.Warnln("no sensors stored; attempting automatic discovery")
		if err = DiscoverFanSensorIDs(); err != nil {
			logging.Errorln("fan sensor auto-discovery failed, unable to query readings")
			return nil, err
		}
	}

	// valid sensor data exists; get readings
	values = make(map[string]int16)
	for id, data := range fanData {
		logging.Debugf("reading data for sensor %d", id)
		if r, err := client.GetSensorReading(ctx, id); err != nil {
			// this isn't a critical error, so log it and continue
			logging.Errorf("failed to query BMC for fan sensor with ID %d", id)
			logging.Errorln(err)
			continue
		} else {
			logging.Debugf("read raw value %d", r.Reading)
			values[data.Name] = int16(r.Reading)*data.M + (data.B*10^int16(data.K1))*10 ^ int16(data.K2)
		}
	}

	return values, nil
}

// EnableManualControl sends a raw IPMI command to the BMC to switch the fan
// mode to manual control.
//
// If the command generates an error, it is returned in err.
func EnableManualControl() (err error) {
	logging.Debugln("entering EnableManualControl")
	defer logging.Debugln("exiting EnableManualControl")

	client.Connect(ctx)
	defer client.Close(ctx)

	logging.Debugln("SetManual raw request bytes:")
	logging.Debugln(append([]uint8{uint8(ipmi.NetFnOEMSupermicroRequest), fanControl}, manualBytes...))
	if resp, err := client.RawCommand(ctx, ipmi.NetFnOEMSupermicroRequest, fanControl, manualBytes, "SetManual"); err != nil {
		logging.Errorln(err)
		return fmt.Errorf("failed to enable manual fan control")
	} else {
		logging.Debugln("raw response bytes:")
		logging.Debugln(resp.Response)
	}

	return
}

// EnableAutoCurve sends a raw IPMI command to the BMC to switch the fan
// mode to automatic control.
//
// If the command generates an error, it is returned in err.
func EnableAutoCurve() (err error) {
	logging.Debugln("entering EnableAutoCurve")
	defer logging.Debugln("exiting EnableAutoCurve")

	client.Connect(ctx)
	defer client.Close(ctx)

	logging.Debugln("SetAuto raw request bytes:")
	logging.Debugln(append([]uint8{uint8(ipmi.NetFnOEMSupermicroRequest), fanControl}, autoBytes...))
	if resp, err := client.RawCommand(ctx, ipmi.NetFnOEMSupermicroRequest, fanControl, autoBytes, "SetAuto"); err != nil {
		logging.Errorln(err)
		return fmt.Errorf("failed to enable automatic fan control")
	} else {
		logging.Debugln("raw response bytes:")
		logging.Debugln(resp.Response)
	}

	return
}

// SetAllFans updates the speed of all BMC-controlled chassis fans.
//
// The speed should be a value between 0x00 and 0x64, where each increase of
// 0x01 correlates to a 1% of max speed increase. Any error generated by the
// IPMI command is returned in err.
func SetAllFans(speed uint8) (err error) {
	logging.Debugln("entering SetAllFans")
	defer logging.Debugln("exiting SetAllFans")

	return SetFanByID(0xff, speed)
}

// SetFanByID sets the speed of a single fan.
//
// The id is a 0-index number specifying the fan to update; some
// experimentation is required to correlate fan IDs for this command and fan
// speed sensor IDs used elsewhere. The speed should be a value between 0x00
// and 0x64, where each increase of 0x01 correlates to a 1% of max speed
// increase. Any error generated by the IPMI command is returned in err.
func SetFanByID(id uint8, speed uint8) (err error) {
	logging.Debugln("entering SetFanByID")
	defer logging.Debugln("exiting SetFanByID")

	if speed > 0x64 {
		return fmt.Errorf("fan speed must be between 0x00 and 0x64")
	}

	client.Connect(ctx)
	defer client.Close(ctx)

	logging.Debugf("setting fan %d to %d%%", id, speed)
	data := []uint8{setSpeedData, id, speed}
	logging.Debugln("SetFanSpeed raw request bytes:")
	logging.Debugln(append([]uint8{uint8(ipmi.NetFnOEMSupermicroRequest), fanControl}, data...))
	if resp, err := client.RawCommand(ctx, ipmi.NetFnOEMSupermicroRequest, fanControl, data, "SetFanSpeed"); err != nil {
		logging.Errorln(err)
		return fmt.Errorf("failed to set desired fan speed")
	} else {
		logging.Debugln("SetFanSpeed raw response bytes:")
		logging.Debugln(resp.Response)
	}

	return
}
