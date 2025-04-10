package glance

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/shirou/gopsutil/v4/sensors"
)

type cliIntent uint8

const (
	cliIntentVersionPrint cliIntent = iota
	cliIntentServe
	cliIntentConfigValidate
	cliIntentConfigPrint
	cliIntentDiagnose
	cliIntentSensorsPrint
)

type cliOptions struct {
	intent     cliIntent
	configPath string
}

func parseCliOptions() (*cliOptions, error) {
	var args []string

	args = os.Args[1:]
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-v" || args[0] == "version") {
		return &cliOptions{
			intent: cliIntentVersionPrint,
		}, nil
	}

	flags := flag.NewFlagSet("", flag.ExitOnError)
	flags.Usage = func() {
		fmt.Println("Usage: glance [options] command")

		fmt.Println("\nOptions:")
		flags.PrintDefaults()

		fmt.Println("\nCommands:")
		fmt.Println("  config:validate     Validate the config file")
		fmt.Println("  config:print        Print the parsed config file with embedded includes")
		fmt.Println("  sensors:print       List all sensors")
		fmt.Println("  diagnose            Run diagnostic checks")
	}
	configPath := flags.String("config", "glance.yml", "Set config path")
	err := flags.Parse(os.Args[1:])
	if err != nil {
		return nil, err
	}

	var intent cliIntent
	args = flags.Args()
	unknownCommandErr := fmt.Errorf("unknown command: %s", strings.Join(args, " "))

	if len(args) == 0 {
		intent = cliIntentServe
	} else if len(args) == 1 {
		if args[0] == "config:validate" {
			intent = cliIntentConfigValidate
		} else if args[0] == "config:print" {
			intent = cliIntentConfigPrint
		} else if args[0] == "sensors:print" {
			intent = cliIntentSensorsPrint
		} else if args[0] == "diagnose" {
			intent = cliIntentDiagnose
		} else {
			return nil, unknownCommandErr
		}
	} else {
		return nil, unknownCommandErr
	}

	return &cliOptions{
		intent:     intent,
		configPath: *configPath,
	}, nil
}

func cliSensorsPrint() int {
	tempSensors, err := sensors.SensorsTemperatures()
	if err != nil {
		fmt.Printf("Failed to retrieve list of sensors: %v\n", err)
		return 1
	}

	if len(tempSensors) == 0 {
		fmt.Println("No sensors found")
		return 0
	}

	for _, sensor := range tempSensors {
		fmt.Printf("%s: %.1f°C\n", sensor.SensorKey, sensor.Temperature)
	}

	return 0
}
