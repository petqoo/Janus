package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pitko/Janus/internal/config"
	"github.com/pitko/Janus/internal/ui"
)

var stableApp *exec.Cmd
 var buildLock sync.Mutex

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
	debounceDuration := 50 * time.Millisecond

	for {
		select {
		case <-func() <-chan time.Time {
			if timer == nil {
				return nil
			}
			return timer.C
		}():
	    	defer buildLock.Unlock()
			timer = nil
			if !buildLock.TryLock() {

				sub <- ui.AegisLogMsg("Build already in progress. Ignoring trigger.")
				continue
			}
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
	defer buildLock.Unlock()

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
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			sub <- ui.AppLogMsg(scanner.Text())
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			sub <- ui.AppLogMsg(scanner.Text())
		}
	}()

	return cmd, nil
}

func stopProcess(sub chan any) {
	if stableApp == nil || stableApp.Process == nil {
		return
	}
	pid := stableApp.Process.Pid
	if sub == nil {
		syscall.Kill(-pid, syscall.SIGKILL)
		return
	}
	sub <- ui.AegisLogMsg("Stopping stable process...")
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		sub <- ui.AegisLogMsg(fmt.Sprintf("Error sending SIGTERM (%v), forcing kill.", err))
		syscall.Kill(-pid, syscall.SIGKILL)
		return
	}

	done := make(chan error)
	go func() {
		done <- stableApp.Wait()
	}()

	select {
	case <-time.After(2 * time.Second):
		sub <- ui.AegisLogMsg("Process did not stop gracefully, forcing kill.")
		syscall.Kill(-pid, syscall.SIGKILL)
	case <-done:
		sub <- ui.AegisLogMsg("Process stopped gracefully.")
	}
	stableApp = nil
}

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

func isExcluded(dirName string, excludedDirs []string) bool {
	for _, excluded := range excludedDirs {
		if dirName == excluded {
			return true
		}
	}
	return false
}