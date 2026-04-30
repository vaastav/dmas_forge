package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/vaastav/agentic_blueprint/benchmark/runner/execute"
)

type config struct {
	Mock     bool                    `json:"mock"`
	Profiles []execute.Profile       `json:"profiles"`
	Examples []execute.ExampleConfig `json:"examples"`
}

func loadConfig(path string) (config, error) {
	var cfg config
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, validateConfig(cfg)
}

func validateConfig(cfg config) error {
	for _, profile := range cfg.Profiles {
		if err := validateProfile("default", profile); err != nil {
			return err
		}
	}
	for _, ex := range cfg.Examples {
		for _, profile := range ex.Profiles {
			if err := validateProfile(ex.Name, profile); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateProfile(source string, profile execute.Profile) error {
	switch profile.Mode {
	case "requests", "timed":
	default:
		return fmt.Errorf("%s profile %q has unsupported mode %q", source, profile.Name, profile.Mode)
	}
	if profile.Value < 1 {
		return fmt.Errorf("%s profile %q value must be at least 1", source, profile.Name)
	}
	return nil
}

func selectCases(cfg config, examples, specs, profiles map[string]bool) []execute.CasePlan {
	var cases []execute.CasePlan
	for _, ex := range cfg.Examples {
		if len(examples) > 0 && !examples[ex.Name] {
			continue
		}
		exProfiles := cfg.Profiles
		if len(ex.Profiles) > 0 {
			exProfiles = ex.Profiles
		}
		for _, spec := range ex.Specs {
			if len(specs) > 0 && !specs[spec] {
				continue
			}
			for _, profile := range exProfiles {
				if len(profiles) > 0 && !profiles[profile.Name] {
					continue
				}
				cases = append(cases, execute.CasePlan{Example: ex, Spec: spec, Profile: profile})
			}
		}
	}
	return cases
}

func profileNames(profiles []execute.Profile) string {
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		names = append(names, profile.Name)
	}
	return strings.Join(names, ",")
}

func printProfile(profile execute.Profile) {
	fmt.Printf("  %-12s mode=%s value=%d concurrency=%d timeout=%ds\n", profile.Name, profile.Mode, profile.Value, profile.Concurrency, profile.TimeoutSeconds)
}
