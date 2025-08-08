package main

import (
	"fmt"

	"github.com/bmoller/ipmi-fancontrol/coretemp"
)

func main() {
	/*
		if values, err := ipmi.GetFanSpeeds(); err != nil {
			fmt.Println(err)
		} else {
			for name, speed := range values {
				fmt.Printf("%s: %d\n", name, speed)
			}
		}
		if temp, err := ipmi.GetAmbientTemperature(); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Ambient Temperature: %d\n", temp)
		}
	*/

	if t, err := coretemp.MaxTemperature(); err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(t)
	}
}
