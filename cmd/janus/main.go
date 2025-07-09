package main

import (
	"fmt"
	"log"
	"os"

	"github.com/charmbracelet/bubbletea"
	"github.com/pitko/Janus/internal/config"
	"github.com/pitko/Janus/internal/ui"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := config.Init(); err != nil {
			log.Fatalf("Failed to create config file: %v", err)
		}
		fmt.Println("'.aegis.toml' created successfully!")
		return
	}

	cfg, err := config.Load(".aegis.toml")
	if err != nil {
		fmt.Printf("Error: Could not load .aegis.toml file. Please run 'aegis init' first.\nError details: %v\n", err)
		os.Exit(1)
	}

	msgChannel := make(chan any)
	model := ui.InitialModel(msgChannel)
	program := tea.NewProgram(model, tea.WithAltScreen()) 

	go StartWatcher(cfg, msgChannel)

	if  err := program.Start(); err != nil {
		fmt.Printf("Error starting TUI: %v\n", err)
		os.Exit(1)
	}
	stopProcess(nil)
	
	os.Remove(cfg.Build.Bin)
}