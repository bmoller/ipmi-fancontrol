package ipmi

import (
	"context"
	"regexp"
	"time"

	openipmi "github.com/bougou/go-ipmi"

	"github.com/bmoller/ipmi-fancontrol/logging"
)

const (
	ambientSensorName = "Ambient Temp"
	fanSensorNameRE   = `^FAN \d+ RPM$`
	fiveSeconds       = "5s"
)

var (
	client        *openipmi.Client
	ctx           context.Context
	fanSensorName *regexp.Regexp
	retryInterval time.Duration
)

func init() {
	var err error
	if retryInterval, err = time.ParseDuration(fiveSeconds); err != nil {
		panic("failed to parse interval as a duration")
	}
	for client, err = openipmi.NewOpenClient(); err != nil; client, err = openipmi.NewOpenClient() {
		logging.Errorln("failed to to create OpenIPMI client:")
		logging.Errorln(err)
		time.Sleep(retryInterval)
	}
	fanSensorName = regexp.MustCompile(fanSensorNameRE)
	ctx = context.Background()
}

func openConnection() {
	for err := client.Connect(ctx); err != nil; err = client.Connect(ctx) {
		logging.Errorln("failed to connect to IPMI socket")
		logging.Errorln(err)
		time.Sleep(retryInterval)
	}
}

func GetAmbientTemperature() (t int, err error) {
	openConnection()
	defer client.Close(ctx)

	if sensor, err := client.GetSensorByName(ctx, ambientSensorName); err != nil {
		logging.Errorln("failed to read ambient temperature")
		return t, err
	} else {
		t = int(sensor.Value)
	}

	return
}

func GetFanSpeeds() (values map[string]int, err error) {
	openConnection()
	defer client.Close(ctx)

	if sensors, err := client.GetSensors(ctx, openipmi.SensorFilterOptionIsSensorType(openipmi.SensorTypeFan)); err != nil {
		logging.Errorln("failed to query BMC for fan sensors")
		return nil, err
	} else {
		values = make(map[string]int)
		for _, sensor := range sensors {
			if fanSensorName.MatchString(sensor.Name) {
				values[sensor.Name] = int(sensor.Value)
			}
		}
	}

	return
}
