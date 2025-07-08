package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pitko/Janus/internal/config"
)

var (
	stableApp *exec.Cmd
	cfg       *config.Config
)

func startProcess(binPath string) (*exec.Cmd, error) {
	cmd := exec.Command(binPath)
	cmd.Dir = cfg.Watch.Root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	return cmd, err
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := config.Init(); err != nil {
			log.Fatalf("Failed to create config file: %v", err)
		}
		fmt.Println("'.aegis.toml' created successfully!")
		return
	}

	var err error
	cfg, err = config.Load(".aegis.toml")
	if err != nil {
		log.Fatalf("Error loading .aegis.toml: %v. Run 'aegis init' to create one.", err)
	}

	if !buildAndRun(true) {
		log.Fatalf("Initial build failed. Please fix errors and restart.")
	}

	go watchFiles()

	waitForSignal()
}

func stopProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	log.Println("Politely stopping process...")
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("Error sending SIGTERM, forcing kill: %v", err)
		cmd.Process.Kill()
		return
	}

	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(2 * time.Second):
		log.Println("Process did not stop gracefully, forcing kill.")
		cmd.Process.Kill()
	case <-done:
		log.Println("Process stopped gracefully.")
	}
}

func buildAndRun(isInitial bool) bool {
	challengerBinPath := cfg.Build.Bin
	stableBinPath := challengerBinPath + ".stable"

	stopProcess(stableApp)

	if _, err := os.Stat(challengerBinPath); err == nil {
		log.Println("Preserving last stable version...")
		if err := os.Rename(challengerBinPath, stableBinPath); err != nil {
			log.Printf("Could not preserve stable binary: %v", err)
		}
	}

	log.Println("Building ...")
	buildCmd := exec.Command("sh", "-c", cfg.Build.Cmd)
	buildCmd.Dir = cfg.Watch.Root
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		log.Printf("Build failed: %v", err)
		revertToStable("Build failed, restoring previous version.")
		return false
	}

	log.Println("Starting challenger on probation...")
	challenger, err := startProcess(challengerBinPath)
	if err != nil {
		log.Printf("Failed to start challenger: %v", err)
		revertToStable("Challenger failed to start.")
		return false
	}

	done := make(chan error)
	go func() { done <- challenger.Wait() }()

	select {
	case <-time.After(cfg.SafetyNet.Probation):
		log.Println("Challenger survived probation! Promoting to stable.")
		stableApp = challenger
		os.Remove(stableBinPath)
		return true
	case err := <-done:
		log.Printf("Challenger crashed during probation: %v", err)
		revertToStable("Challenger crashed on startup.")
		return false
	}
}

func revertToStable(reason string) {
	log.Printf("Reverting to last stable version: %s", reason)

	challengerBinPath := cfg.Build.Bin
	stableBinPath := challengerBinPath + ".stable"

	os.Remove(challengerBinPath)

	if _, err := os.Stat(stableBinPath); os.IsNotExist(err) {
		log.Println("No stable version available to revert to. Application is stopped.")
		stableApp = nil
		return
	}

	if err := os.Rename(stableBinPath, challengerBinPath); err != nil {
		log.Fatalf("CRITICAL: Failed to restore stable binary: %v", err)
		return
	}

	log.Println("Restarting last known stable version...")
	restartedApp, err := startProcess(challengerBinPath)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to restart stable binary: %v", err)
		return
	}
	stableApp = restartedApp
	log.Println("Revert successful. Last stable version is running.")
}

func watchFiles() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create file watcher: %v", err)
	}
	defer watcher.Close()

	filepath.Walk(cfg.Watch.Root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			for _, excluded := range cfg.Watch.ExcludeDir {
				if excluded == info.Name() {
					return filepath.SkipDir
				}
			}
			if err := watcher.Add(path); err != nil {
				log.Printf("Failed to watch directory %s: %v", path, err)
			}
		}
		return nil
	})

	var timer *time.Timer
	debounceDuration := 500 * time.Millisecond

	log.Println("Watching for file changes...")
	for {
		select {
		case <-func() <-chan time.Time {
			if timer == nil {
				return nil
			}
			return timer.C
		}():
			log.Println("Debounce timer fired. Triggering reload...")
			go buildAndRun(false)

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) {
				if timer != nil {
					timer.Reset(debounceDuration)
				} else {
					timer = time.NewTimer(debounceDuration)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

func waitForSignal() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	log.Println("Shutting down...")
	stopProcess(stableApp)
}