package config

import (
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Build    BuildSettings    `toml:"build"`
	Watch    WatchSettings    `toml:"watch"`
	SafetyNet SafetyNetSettings `toml:"safety_net"`
}

type BuildSettings struct {
	Cmd string `toml:"cmd"`
	Bin string `toml:"bin"`
}

type WatchSettings struct {
	Root       string   `toml:"root"`
	ExcludeDir []string `toml:"exclude_dir"`
}

type SafetyNetSettings struct {
	Probation time.Duration `toml:"probation_ms"`
}

func Load(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}
	cfg.SafetyNet.Probation = cfg.SafetyNet.Probation * time.Millisecond
	return &cfg, nil
}

func Init() error {
	defaultConfig := `
# .aegis.toml

# Command to build your Go application.
[build]
  # This command is run from the 'watch.root' directory.
  cmd = "go build -o ./tmp/app ./cmd/janus/main.go"

  # The path to the binary that the command above creates.
  # This path is relative to the 'watch.root' directory.
  bin = "./tmp/app"

# Settings for watching files.
[watch]
  # The application directory you want to watch.
  root = "."
  # Directories to ignore.
  exclude_dir = ["tmp", "vendor", ".git"]

# The crash-proof safety net configuration.
[safety_net]
  # How long to wait in milliseconds to see if the new app crashes.
  # If the app runs for this long, it's considered "stable".
  probation_ms = 2500
`
	return os.WriteFile(".aegis.toml", []byte(defaultConfig), 0644)
}