package ssh

import (
	"fmt"
	"go-ssh/password"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/creack/pty"
)

// Connect executes the SSH command and replaces the current process
func Connect(command string) error {
	if command == "" {
		return fmt.Errorf("no command specified")
	}

	// Parse the command into shell and args
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	// Create command
	cmd := exec.Command(shell, "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error executing SSH command: %w", err)
	}

	return nil
}

// ConnectWithExec replaces the current process with SSH (using exec syscall)
func ConnectWithExec(command string) error {
	if command == "" {
		return fmt.Errorf("no command specified")
	}

	// Get the shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	// Prepare arguments
	args := []string{shell, "-c", command}

	// Execute and replace current process
	env := os.Environ()
	if err := syscall.Exec(shell, args, env); err != nil {
		return fmt.Errorf("error executing SSH command: %w", err)
	}

	// This line will never be reached if Exec succeeds
	return nil
}

// ValidateCommand performs basic validation on the SSH command
func ValidateCommand(command string) error {
	if command == "" {
		return fmt.Errorf("command cannot be empty")
	}

	if !strings.Contains(command, "ssh") {
		return fmt.Errorf("command must contain 'ssh'")
	}

	return nil
}

// ValidateCommands validates a list of commands
func ValidateCommands(commands []string) error {
	if len(commands) == 0 {
		return fmt.Errorf("no commands specified")
	}

	// At least one command should contain SSH
	hasSSH := false
	for _, cmd := range commands {
		if cmd == "" {
			return fmt.Errorf("empty command in command list")
		}
		if strings.Contains(cmd, "ssh") {
			hasSSH = true
		}
	}

	if !hasSSH {
		return fmt.Errorf("at least one command must contain 'ssh'")
	}

	return nil
}

// ConnectWithCommands executes multiple commands in sequence
// It finds the first SSH command and embeds remaining commands as remote commands
// For example: ["ssh host1", "sleep 2", "ssh host2"] becomes "ssh -tt host1 'sleep 2; exec ssh host2'"
func ConnectWithCommands(commands []string) error {
	if len(commands) == 0 {
		return fmt.Errorf("no commands specified")
	}

	// If only one command, use the regular connect
	if len(commands) == 1 {
		return ConnectWithExec(commands[0])
	}

	// Find the first SSH command
	var firstSSHIndex = -1
	for i, cmd := range commands {
		if strings.Contains(cmd, "ssh") {
			firstSSHIndex = i
			break
		}
	}

	if firstSSHIndex == -1 {
		// No SSH command found, just chain them with && and run
		combinedCommand := strings.Join(commands, " && ")
		return ConnectWithExec(combinedCommand)
	}

	firstSSH := commands[firstSSHIndex]

	// If SSH is the last command, just execute it
	if firstSSHIndex == len(commands)-1 {
		return ConnectWithExec(firstSSH)
	}

	// Get commands before first SSH
	var preCommands []string
	if firstSSHIndex > 0 {
		preCommands = commands[:firstSSHIndex]
	}

	// Get commands after first SSH (these will run remotely)
	remoteCommands := commands[firstSSHIndex+1:]

	// Build the remote script
	// Use 'exec' for the last command to replace the shell
	var remoteScript strings.Builder
	for i, cmd := range remoteCommands {
		if i > 0 {
			remoteScript.WriteString("; ")
		}
		// Add 'exec' to the last command if it contains ssh
		if i == len(remoteCommands)-1 && strings.Contains(cmd, "ssh") {
			remoteScript.WriteString("exec ")
		}
		remoteScript.WriteString(cmd)
	}

	// Ensure SSH has -tt flag for proper terminal allocation
	sshCommand := firstSSH
	if !strings.Contains(sshCommand, " -tt") && !strings.Contains(sshCommand, " -t ") {
		// Insert -tt after 'ssh'
		sshCommand = strings.Replace(sshCommand, "ssh ", "ssh -tt ", 1)
	}

	// Escape single quotes in the remote script
	escapedScript := strings.ReplaceAll(remoteScript.String(), "'", "'\"'\"'")

	// Combine: ssh -tt host 'remote commands'
	finalCommand := fmt.Sprintf("%s '%s'", sshCommand, escapedScript)

	// If there are pre-commands, chain them
	if len(preCommands) > 0 {
		preScript := strings.Join(preCommands, " && ")
		finalCommand = fmt.Sprintf("%s && %s", preScript, finalCommand)
	}

	fmt.Fprintf(os.Stdout, "Executing: %s\n", finalCommand)

	return ConnectWithExec(finalCommand)
}

// ConnectWithCommandsSubprocess executes all commands as subprocesses (no exec)
// This is useful when exec is not desired or fails
// Uses the same embed logic as ConnectWithCommands
func ConnectWithCommandsSubprocess(commands []string) error {
	if len(commands) == 0 {
		return fmt.Errorf("no commands specified")
	}

	// If only one command, use the regular connect
	if len(commands) == 1 {
		return Connect(commands[0])
	}

	// Find the first SSH command
	var firstSSHIndex = -1
	for i, cmd := range commands {
		if strings.Contains(cmd, "ssh") {
			firstSSHIndex = i
			break
		}
	}

	if firstSSHIndex == -1 {
		// No SSH command found, just chain them with && and run
		combinedCommand := strings.Join(commands, " && ")
		return Connect(combinedCommand)
	}

	firstSSH := commands[firstSSHIndex]

	// If SSH is the last command, just execute it
	if firstSSHIndex == len(commands)-1 {
		return Connect(firstSSH)
	}

	// Get commands before first SSH
	var preCommands []string
	if firstSSHIndex > 0 {
		preCommands = commands[:firstSSHIndex]
	}

	// Get commands after first SSH (these will run remotely)
	remoteCommands := commands[firstSSHIndex+1:]

	// Build the remote script
	var remoteScript strings.Builder
	for i, cmd := range remoteCommands {
		if i > 0 {
			remoteScript.WriteString("; ")
		}
		// Add 'exec' to the last command if it contains ssh
		if i == len(remoteCommands)-1 && strings.Contains(cmd, "ssh") {
			remoteScript.WriteString("exec ")
		}
		remoteScript.WriteString(cmd)
	}

	// Ensure SSH has -tt flag for proper terminal allocation
	sshCommand := firstSSH
	if !strings.Contains(sshCommand, " -tt") && !strings.Contains(sshCommand, " -t ") {
		// Insert -tt after 'ssh'
		sshCommand = strings.Replace(sshCommand, "ssh ", "ssh -tt ", 1)
	}

	// Escape single quotes in the remote script
	escapedScript := strings.ReplaceAll(remoteScript.String(), "'", "'\"'\"'")

	// Combine: ssh -tt host 'remote commands'
	finalCommand := fmt.Sprintf("%s '%s'", sshCommand, escapedScript)

	// If there are pre-commands, chain them
	if len(preCommands) > 0 {
		preScript := strings.Join(preCommands, " && ")
		finalCommand = fmt.Sprintf("%s && %s", preScript, finalCommand)
	}

	fmt.Fprintf(os.Stdout, "Executing: %s\n", finalCommand)

	return Connect(finalCommand)
}

// CommandType represents the type of command in interactive mode
type CommandType int

const (
	CommandTypeExec     CommandType = iota // Normal command to execute
	CommandTypeSend                        // Send text to stdin (e.g., SEND:password)
	CommandTypeSendPass                    // Send password from store (e.g., SENDPASS:id)
	CommandTypeWait                        // Wait for duration (e.g., WAIT:2)
	CommandTypeInteract                    // Give control to user (e.g., INTERACT)
)

// ParsedCommand represents a parsed command with its type and value
type ParsedCommand struct {
	Type  CommandType
	Value string
}

// ParseCommands parses commands and identifies special prefixes
func ParseCommands(commands []string) []ParsedCommand {
	var parsed []ParsedCommand
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, "SEND:") {
			parsed = append(parsed, ParsedCommand{
				Type:  CommandTypeSend,
				Value: strings.TrimPrefix(cmd, "SEND:"),
			})
		} else if strings.HasPrefix(cmd, "SENDPASS:") {
			parsed = append(parsed, ParsedCommand{
				Type:  CommandTypeSendPass,
				Value: strings.TrimPrefix(cmd, "SENDPASS:"),
			})
		} else if strings.HasPrefix(cmd, "WAIT:") {
			parsed = append(parsed, ParsedCommand{
				Type:  CommandTypeWait,
				Value: strings.TrimPrefix(cmd, "WAIT:"),
			})
		} else if cmd == "INTERACT" || cmd == "INTERACTIVE" {
			parsed = append(parsed, ParsedCommand{
				Type:  CommandTypeInteract,
				Value: "",
			})
		} else {
			parsed = append(parsed, ParsedCommand{
				Type:  CommandTypeExec,
				Value: cmd,
			})
		}
	}
	return parsed
}

// ConnectInteractive executes commands in interactive mode using PTY
// This allows sending automated input (passwords, commands) and then giving control to user
func ConnectInteractive(commands []string) error {
	if len(commands) == 0 {
		return fmt.Errorf("no commands specified")
	}

	parsed := ParseCommands(commands)
	if len(parsed) == 0 {
		return fmt.Errorf("no valid commands")
	}

	// Check if we need password store
	needsPasswordStore := false
	for _, pc := range parsed {
		if pc.Type == CommandTypeSendPass {
			needsPasswordStore = true
			break
		}
	}

	var passwordStore *password.PasswordStore
	if needsPasswordStore {
		passwordStore = password.NewPasswordStore()

		// Check if password store exists
		if !passwordStore.StoreExists() {
			return fmt.Errorf("password store not initialized. Please run password manager to add passwords first")
		}

		// Prompt for master password
		masterPassword, err := password.PromptMasterPassword("Master Password: ")
		if err != nil {
			return fmt.Errorf("failed to read master password: %w", err)
		}

		// Load password store
		if err := passwordStore.Load(masterPassword); err != nil {
			return fmt.Errorf("failed to load password store: %w", err)
		}

		fmt.Println("Password store loaded successfully")
	}

	// Find first exec command (should be SSH)
	var execCmd string
	var startIdx int
	for i, pc := range parsed {
		if pc.Type == CommandTypeExec {
			execCmd = pc.Value
			startIdx = i + 1
			break
		}
	}

	if execCmd == "" {
		return fmt.Errorf("no executable command found")
	}

	// Get shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	// Create command
	cmd := exec.Command(shell, "-c", execCmd)

	// Start with a pty
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start pty: %w", err)
	}
	defer func() { _ = ptmx.Close() }()

	// Handle window size changes
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				// Ignore errors during resize
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize

	// Set stdin in raw mode for proper terminal behavior
	oldState, err := MakeRaw(os.Stdin.Fd())
	if err != nil {
		return fmt.Errorf("failed to set raw mode: %w", err)
	}
	defer func() { _ = Restore(os.Stdin.Fd(), oldState) }()

	// Create a filtered reader to remove terminal control sequences
	filteredOutput := &TerminalFilter{Reader: ptmx}

	// Process automation commands
	go func() {
		time.Sleep(500 * time.Millisecond) // Give initial command time to start

		for _, pc := range parsed[startIdx:] {
			switch pc.Type {
			case CommandTypeSend:
				// Send text followed by newline
				fmt.Fprintf(ptmx, "%s\n", pc.Value)
				time.Sleep(100 * time.Millisecond)

			case CommandTypeSendPass:
				// Get password from store and send it
				if passwordStore == nil {
					fmt.Fprintf(os.Stderr, "Error: password store not loaded\n")
					return
				}

				pwd, err := passwordStore.Get(pc.Value)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: failed to get password '%s': %v\n", pc.Value, err)
					return
				}

				// Send password followed by newline
				fmt.Fprintf(ptmx, "%s\n", pwd)
				time.Sleep(100 * time.Millisecond)

			case CommandTypeWait:
				// Parse duration and wait
				var seconds int
				fmt.Sscanf(pc.Value, "%d", &seconds)
				if seconds > 0 {
					time.Sleep(time.Duration(seconds) * time.Second)
				}

			case CommandTypeInteract:
				// User interaction - copy stdin to pty
				go io.Copy(ptmx, os.Stdin)
				return

			case CommandTypeExec:
				// Execute another command
				fmt.Fprintf(ptmx, "%s\n", pc.Value)
				time.Sleep(100 * time.Millisecond)
			}
		}

		// After all automation, give control to user
		io.Copy(ptmx, os.Stdin)
	}()

	// Copy output from pty to stdout (with filtering)
	io.Copy(os.Stdout, filteredOutput)

	// Wait for command to finish
	if err := cmd.Wait(); err != nil {
		// SSH connections often exit with non-zero, ignore
		return nil
	}

	return nil
}

// MakeRaw puts the terminal into raw mode
func MakeRaw(fd uintptr) (*syscall.Termios, error) {
	termios, err := getTermios(fd)
	if err != nil {
		return nil, err
	}

	oldState := *termios
	termios.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP | syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	termios.Oflag &^= syscall.OPOST
	termios.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	termios.Cflag &^= syscall.CSIZE | syscall.PARENB
	termios.Cflag |= syscall.CS8

	if err := setTermios(fd, termios); err != nil {
		return nil, err
	}

	return &oldState, nil
}

// Restore restores the terminal to its previous state
func Restore(fd uintptr, termios *syscall.Termios) error {
	return setTermios(fd, termios)
}

func getTermios(fd uintptr) (*syscall.Termios, error) {
	termios := &syscall.Termios{}
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TIOCGETA, uintptr(unsafe.Pointer(termios)), 0, 0, 0)
	if err != 0 {
		return nil, err
	}
	return termios, nil
}

func setTermios(fd uintptr, termios *syscall.Termios) error {
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TIOCSETA, uintptr(unsafe.Pointer(termios)), 0, 0, 0)
	if err != 0 {
		return err
	}
	return nil
}

// TerminalFilter filters out unwanted terminal control sequences
type TerminalFilter struct {
	Reader io.Reader
	buffer []byte
}

// Read implements io.Reader with filtering
func (tf *TerminalFilter) Read(p []byte) (n int, err error) {
	// Read from source
	n, err = tf.Reader.Read(p)
	if n == 0 {
		return n, err
	}

	// Append to buffer for stateful parsing
	tf.buffer = append(tf.buffer, p[:n]...)

	// Filter out terminal control sequences that shouldn't be displayed
	filtered := make([]byte, 0, len(tf.buffer))
	i := 0
	for i < len(tf.buffer) {
		// Check for ESC[ sequences (CSI - Control Sequence Introducer)
		if i+1 < len(tf.buffer) && tf.buffer[i] == 0x1b && tf.buffer[i+1] == '[' {
			// Found ESC[, scan for the terminating character
			j := i + 2

			// CSI sequences: ESC [ <parameters> <final byte>
			// Parameters are digits, semicolons, and sometimes other chars
			// Final byte is typically a letter or specific symbol
			for j < len(tf.buffer) &&
				((tf.buffer[j] >= '0' && tf.buffer[j] <= '9') ||
					tf.buffer[j] == ';' ||
					tf.buffer[j] == '?' ||
					tf.buffer[j] == '=' ||
					tf.buffer[j] == '>' ||
					tf.buffer[j] == '!' ||
					tf.buffer[j] == ' ') {
				j++
			}

			// Check if we found a terminator
			if j < len(tf.buffer) {
				terminator := tf.buffer[j]
				// Common terminal query responses to filter:
				// ESC[...R (cursor position report)
				// ESC[...c (device attributes)
				// ESC[...n (device status report)
				// But keep normal display sequences like ESC[...m (colors)
				if terminator == 'R' || terminator == 'c' || terminator == 'n' {
					// Skip this sequence
					i = j + 1
					continue
				} else if terminator >= 0x40 && terminator <= 0x7E {
					// Valid CSI terminator - keep it (like colors, cursor movements, etc.)
					for k := i; k <= j; k++ {
						filtered = append(filtered, tf.buffer[k])
					}
					i = j + 1
					continue
				}
			}
			// If no terminator found, keep buffering (sequence might be incomplete)
			if j >= len(tf.buffer) {
				tf.buffer = tf.buffer[i:]
				break
			}
		}

		// Check for partial sequences (just ;numberR or ;number without ESC)
		if tf.buffer[i] == ';' && i+1 < len(tf.buffer) {
			j := i + 1
			hasDigits := false
			for j < len(tf.buffer) && tf.buffer[j] >= '0' && tf.buffer[j] <= '9' {
				hasDigits = true
				j++
			}
			if hasDigits && j < len(tf.buffer) && (tf.buffer[j] == 'R' || tf.buffer[j] == 'c') {
				// Found partial control sequence, skip it
				i = j + 1
				continue
			}
		}

		filtered = append(filtered, tf.buffer[i])
		i++
	}

	// Clear buffer after processing
	tf.buffer = nil

	// Copy filtered data back
	n = copy(p, filtered)
	return n, err
}
