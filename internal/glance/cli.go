package glance

import (
	"flag"
	"os"
)

type CliIntent uint8

const (
	CliIntentServe       CliIntent = iota
	CliIntentCheckConfig           = iota
)

type CliOptions struct {
	Intent     CliIntent
	ConfigPath string
}

func ParseCliOptions() (*CliOptions, error) {
	flags := flag.NewFlagSet("", flag.ExitOnError)

	checkConfig := flags.Bool("check-config", false, "Check whether the config is valid")
	configPath := flags.String("config", "glance.yml", "Set config path")

	err := flags.Parse(os.Args[1:])

	if err != nil {
		return nil, err
	}

	intent := CliIntentServe

	if *checkConfig {
		intent = CliIntentCheckConfig
	}

	return &CliOptions{
		Intent:     intent,
		ConfigPath: *configPath,
	}, nil
}
