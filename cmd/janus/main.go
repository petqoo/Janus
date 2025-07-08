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

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := config.Init(); err != nil {
			log.Fatalf(" Failed to create config file: %v", err)
		}
		fmt.Println(" '.aegis.toml' created successfully!")
		return
	}

	var err error
	cfg, err = config.Load(".aegis.toml")
	if err != nil {
		log.Fatalf("Error loading .aegis.toml: %v. Run 'aegis init' to create one.", err)
	}

	if !buildAndRun(true) {
		log.Fatalf(" Initial build failed. Please fix errors and restart.")
	}

	go watchFiles()

	waitForSignal()
}

func buildAndRun(isInitial bool) bool {
	log.Println(" Building ...")
	buildCmd := exec.Command("sh", "-c", cfg.Build.Cmd)
	buildCmd.Dir = cfg.Watch.Root
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		log.Printf(" Build failed: %v", err)
		return false
	}

	log.Println(" Starting challenger on probation...")
	if stableApp != nil && stableApp.Process != nil {
		if err := stableApp.Process.Kill(); err != nil {
			log.Printf(" Could not stop stable app: %v", err)
		}
	}

	challenger := exec.Command(cfg.Build.Bin)
	challenger.Dir = cfg.Watch.Root
	challenger.Stdout = os.Stdout
	challenger.Stderr = os.Stderr
	if err := challenger.Start(); err != nil {
		log.Printf(" Failed to start challenger: %v", err)
		revertToStable("Challenger failed to start.")
		return false
	}

	done := make(chan error)
	go func() { done <- challenger.Wait() }()

	select {
	case <-time.After(cfg.SafetyNet.Probation):
		log.Println("Challenger survived probation! Promoting to stable.")
		stableApp = challenger
		return true
	case err := <-done:
		log.Printf(" Challenger crashed during probation: %v", err)
		revertToStable("Challenger crashed on startup.")
		return false
	}
}

func revertToStable(reason string) {
	log.Printf(" Reverting to last stable version: %s", reason)
	if stableApp != nil {
		log.Println(" Last stable version is running (simulation).")
	} else {
		log.Println(" No stable version available to revert to.")
	}
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

    log.Println(" Watching for file changes...")
    for {
        select {
        case event, ok := <-watcher.Events:
            if !ok {
                return
            }
            if event.Has(fsnotify.Write) {
                log.Printf(" File modified: %s. Triggering reload...", event.Name)
                buildAndRun(false)
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
	if stableApp != nil && stableApp.Process != nil {
		stableApp.Process.Kill()
	}
}