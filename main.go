package main

import (
	"flag"
	"fmt"
	"go-ssh/config"
	"go-ssh/password"
	"go-ssh/ssh"
	"go-ssh/ui"
	"os"
	"strings"
)

func main() {
	// Parse command line flags
	passwordMode := flag.Bool("passwords", false, "Manage stored passwords")
	flag.Parse()

	// Password manager mode
	if *passwordMode {
		runPasswordManager()
		return
	}

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Check if there are any categories configured
	if len(cfg.Categories) == 0 {
		fmt.Fprintf(os.Stderr, "No hosts configured. Please add hosts to ~/.go-ssh/config.yaml\n")
		os.Exit(1)
	}

	// Run the TUI and get selected host
	selectedHost, err := ui.Run(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running UI: %v\n", err)
		os.Exit(1)
	}

	// If no host selected (user quit), exit gracefully
	if selectedHost == nil {
		return
	}

	// Get commands from the selected host
	commands := selectedHost.GetCommands()
	if len(commands) == 0 {
		fmt.Fprintf(os.Stderr, "No command configured for host: %s\n", selectedHost.Name)
		os.Exit(1)
	}

	// Validate the commands
	if len(commands) == 1 {
		if err := ssh.ValidateCommand(commands[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid command: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := ssh.ValidateCommands(commands); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid commands: %v\n", err)
			os.Exit(1)
		}
	}

	// Connect to the selected host
	// Check if commands contain special interactive prefixes
	hasInteractive := false
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, "SEND:") || strings.HasPrefix(cmd, "SENDPASS:") ||
			strings.HasPrefix(cmd, "WAIT:") || strings.HasPrefix(cmd, "EXPECT:") ||
			cmd == "INTERACT" || cmd == "INTERACTIVE" {
			hasInteractive = true
			break
		}
	}

	if hasInteractive {
		// Use interactive mode (PTY-based automation)
		if err := ssh.ConnectInteractive(commands); err != nil {
			fmt.Fprintf(os.Stderr, "Error in interactive session: %v\n", err)
			os.Exit(1)
		}
	} else if len(commands) == 1 {
		// Single command - use existing behavior
		if err := ssh.ConnectWithExec(commands[0]); err != nil {
			// If exec fails, try running as subprocess
			fmt.Fprintf(os.Stderr, "Warning: exec failed, running as subprocess: %v\n", err)
			if err := ssh.Connect(commands[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error connecting to host: %v\n", err)
				os.Exit(1)
			}
		}
	} else {
		// Multiple commands - execute sequentially
		if err := ssh.ConnectWithCommands(commands); err != nil {
			// If exec fails on last command, try running all as subprocesses
			fmt.Fprintf(os.Stderr, "Warning: exec failed, running as subprocess: %v\n", err)
			if err := ssh.ConnectWithCommandsSubprocess(commands); err != nil {
				fmt.Fprintf(os.Stderr, "Error executing commands: %v\n", err)
				os.Exit(1)
			}
		}
	}
}

func runPasswordManager() {
	store := password.NewPasswordStore()

	// Check if password store exists
	if !store.StoreExists() {
		fmt.Println("Password store not found. Creating new store...")

		// Prompt for master password
		masterPassword, err := password.PromptMasterPassword("Create Master Password: ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
			os.Exit(1)
		}

		if len(masterPassword) < 8 {
			fmt.Fprintf(os.Stderr, "Master password must be at least 8 characters\n")
			os.Exit(1)
		}

		// Confirm master password
		confirmPassword, err := password.PromptMasterPassword("Confirm Master Password: ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
			os.Exit(1)
		}

		if masterPassword != confirmPassword {
			fmt.Fprintf(os.Stderr, "Passwords do not match\n")
			os.Exit(1)
		}

		// Initialize store
		if err := store.Initialize(masterPassword); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing password store: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Password store created at: %s\n", store.GetStorePath())

		// Run password manager
		if err := ui.RunPasswordManager(store, masterPassword); err != nil {
			fmt.Fprintf(os.Stderr, "Error running password manager: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Password store exists, prompt for master password
	masterPassword, err := password.PromptMasterPassword("Master Password: ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
		os.Exit(1)
	}

	// Load store
	if err := store.Load(masterPassword); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading password store: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Password store loaded successfully")

	// Run password manager
	if err := ui.RunPasswordManager(store, masterPassword); err != nil {
		fmt.Fprintf(os.Stderr, "Error running password manager: %v\n", err)
		os.Exit(1)
	}
}
