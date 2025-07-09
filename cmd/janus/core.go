package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pitko/Janus/internal/config"
	"github.com/pitko/Janus/internal/ui"
)

var stableApp *exec.Cmd

func StartWatcher(cfg *config.Config, sub chan any) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		sub <- ui.ErrorMsg{Err: err}
		return
	}
	defer watcher.Close()

	filepath.Walk(cfg.Watch.Root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() && !isExcluded(info.Name(), cfg.Watch.ExcludeDir) {
			watcher.Add(path)
		}
		return nil
	})

	var timer *time.Timer
	debounceDuration := 500 * time.Millisecond

	for {
		select {
		case <-func() <-chan time.Time {
			if timer == nil {
				return nil
			}
			return timer.C
		}():
			timer = nil
			sub <- ui.AegisLogMsg("Debounce timer fired. Triggering reload...")
			go buildAndRun(cfg, sub) 

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) {
				sub <- ui.AegisLogMsg("File modified: " + event.Name)
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
			sub <- ui.ErrorMsg{Err: err}
		}
	}
}

func buildAndRun(cfg *config.Config, sub chan any) {
	challengerBinPath := cfg.Build.Bin
	stableBinPath := challengerBinPath + ".stable"

	stopProcess(sub)

	if _, err := os.Stat(challengerBinPath); err == nil {
		sub <- ui.AegisLogMsg("Preserving last stable version...")
		if err := os.Rename(challengerBinPath, stableBinPath); err != nil {
			sub <- ui.ErrorMsg{Err: err}
		}
	}

	sub <- ui.StatusMsg("Building...")
	buildCmd := exec.Command("sh", "-c", cfg.Build.Cmd)
	buildCmd.Dir = cfg.Watch.Root
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		sub <- ui.ErrorMsg{Err: err}
		sub <- ui.AegisLogMsg("Build failed:\n" + string(output))
		revertToStable(cfg, sub)
		return
	}

	sub <- ui.AegisLogMsg("Build successful.")
	sub <- ui.StatusMsg("Starting app on probation...")

	challenger, err := startProcess(challengerBinPath, sub, cfg)
	if err != nil {
		sub <- ui.ErrorMsg{Err: err}
		revertToStable(cfg, sub)
		return
	}

	done := make(chan error)
	go func() { done <- challenger.Wait() }()

	select {
	case <-time.After(cfg.SafetyNet.Probation):
		sub <- ui.AegisLogMsg("Probation successful.")
		sub <- ui.StatusMsg("App running (Stable)")
		stableApp = challenger
		os.Remove(stableBinPath) 
	case err := <-done:
		sub <- ui.ErrorMsg{Err: err}
		sub <- ui.AegisLogMsg("Challenger crashed during probation.")
		revertToStable(cfg, sub)
	}
}

func startProcess(binPath string, sub chan any, cfg *config.Config) (*exec.Cmd, error) {
	cmd := exec.Command(binPath)
	cmd.Dir = cfg.Watch.Root

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	// Goroutine to stream stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			sub <- ui.AppLogMsg(scanner.Text())
		}
	}()

	// Goroutine to stream stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			sub <- ui.AppLogMsg(scanner.Text())
		}
	}()

	return cmd, nil
}

// stopProcess gracefully stops the stable application.
func stopProcess(sub chan any) {
	if stableApp == nil || stableApp.Process == nil {
		return
	}

	sub <- ui.AegisLogMsg("Stopping stable process...")
	if err := stableApp.Process.Signal(syscall.SIGTERM); err != nil {
		sub <- ui.AegisLogMsg("Error sending SIGTERM, forcing kill.")
		stableApp.Process.Kill()
		return
	}

	done := make(chan error)
	go func() {
		done <- stableApp.Wait()
	}()

	select {
	case <-time.After(2 * time.Second):
		sub <- ui.AegisLogMsg("Process did not stop gracefully, forcing kill.")
		stableApp.Process.Kill()
	case <-done:
		sub <- ui.AegisLogMsg("Process stopped gracefully.")
	}
	stableApp = nil
}

// revertToStable restores the last known good binary and restarts it.
func revertToStable(cfg *config.Config, sub chan any) {
	sub <- ui.StatusMsg("Reverting to stable version...")
	sub <- ui.AegisLogMsg("Reverting to last stable version...")

	challengerBinPath := cfg.Build.Bin
	stableBinPath := challengerBinPath + ".stable"

	os.Remove(challengerBinPath)

	if _, err := os.Stat(stableBinPath); os.IsNotExist(err) {
		sub <- ui.ErrorMsg{Err: err}
		sub <- ui.StatusMsg("No stable version to revert to.")
		stableApp = nil
		return
	}

	if err := os.Rename(stableBinPath, challengerBinPath); err != nil {
		sub <- ui.ErrorMsg{Err: err}
		sub <- ui.StatusMsg("Could not restore stable binary.")
		return
	}

	restartedApp, err := startProcess(challengerBinPath, sub, cfg)
	if err != nil {
		sub <- ui.ErrorMsg{Err: err}
		sub <- ui.StatusMsg("Failed to restart stable binary.")
		return
	}
	stableApp = restartedApp
	sub <- ui.AegisLogMsg("Revert successful.")
	sub <- ui.StatusMsg("App running (Stable)")
}

// isExcluded is a helper to check if a directory should be ignored.
func isExcluded(dirName string, excludedDirs []string) bool {
	for _, excluded := range excludedDirs {
		if dirName == excluded {
			return true
		}
	}
	return false
}