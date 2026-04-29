package benchmark

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func commandList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	configPath := fs.String("config", "config.json", "config file")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: go run ./benchmark list [-config config.json]")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	_, benchDir, err := repoPaths()
	if err != nil {
		return err
	}
	cfg, err := loadConfig(filepath.Join(benchDir, *configPath))
	if err != nil {
		return err
	}
	for _, ex := range cfg.Examples {
		profiles := cfg.Profiles
		profileSource := "default"
		if len(ex.Profiles) > 0 {
			profiles = ex.Profiles
			profileSource = "custom"
		}
		fmt.Printf("%-20s specs=%-28s profiles=%-24s query=%s\n", ex.Name, strings.Join(ex.Specs, ","), profileNames(profiles)+" ("+profileSource+")", ex.QueryFile)
	}
	fmt.Println()
	fmt.Println("default profiles:")
	for _, profile := range cfg.Profiles {
		printProfile(profile)
	}
	for _, ex := range cfg.Examples {
		if len(ex.Profiles) == 0 {
			continue
		}
		fmt.Printf("%s profiles:\n", ex.Name)
		for _, profile := range ex.Profiles {
			printProfile(profile)
		}
	}
	return nil
}
