package glance

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

type cliIntent uint8

const (
	cliIntentServe          cliIntent = iota
	cliIntentConfigValidate           = iota
	cliIntentConfigPrint              = iota
	cliIntentDiagnose                 = iota
)

type cliOptions struct {
	intent     cliIntent
	configPath string
}

func parseCliOptions() (*cliOptions, error) {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	flags.Usage = func() {
		fmt.Println("Usage: glance [options] command")

		fmt.Println("\nOptions:")
		flags.PrintDefaults()

		fmt.Println("\nCommands:")
		fmt.Println("  config:validate     Validate the config file")
		fmt.Println("  config:print        Print the parsed config file with embedded includes")
		fmt.Println("  diagnose            Run diagnostic checks")
	}
	configPath := flags.String("config", "glance.yml", "Set config path")
	err := flags.Parse(os.Args[1:])
	if err != nil {
		return nil, err
	}

	var intent cliIntent
	var args = flags.Args()
	unknownCommandErr := fmt.Errorf("unknown command: %s", strings.Join(args, " "))

	if len(args) == 0 {
		intent = cliIntentServe
	} else if len(args) == 1 {
		if args[0] == "config:validate" {
			intent = cliIntentConfigValidate
		} else if args[0] == "config:print" {
			intent = cliIntentConfigPrint
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
