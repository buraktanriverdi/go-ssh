go install
go-ssh
# Go SSH Host Manager

A full-screen TUI (terminal UI) application written in Go for organizing SSH hosts in a tree structure and connecting to them quickly.

## Features

- 🎨 Full-screen terminal user interface (TUI)
- 🌳 Tree-based category and host organization
- 📁 Nested categories support
- ⌨️ Keyboard navigation with arrow keys or Vim-style keys
- 🔗 SSH connections via multiple hosts (jump hosts)
- 📜 Sequential command execution for complex connection scenarios
- 🤖 Interactive mode for automatic password entry and command sending
- 📝 YAML-based configuration
- 🏠 Automatic config management under the user home directory

## Installation

```bash
go build -o go-ssh
sudo mv go-ssh /usr/local/bin/
```

or:

```bash
go install
```

## Usage

Run the application:

```bash
go-ssh
```

On first run, the config file `~/.go-ssh/config.yaml` will be created automatically.

### Keyboard Shortcuts

| Key              | Action                            |
|------------------|-----------------------------------|
| `↑/↓` or `j/k`   | Navigate up/down                  |
| `←/→` or `h/l`   | Collapse/expand category          |
| `Enter` or `Space` | Open/close category or connect to host |
| `e`              | Expand all categories             |
| `c`              | Collapse all categories           |
| `q` or `Ctrl+C`  | Quit                              |

## Configuration

Config file path: `~/.go-ssh/config.yaml`

### Tree Structure

Categories can be nested. Each category can contain both subcategories and hosts:

```yaml
categories:
  - name: Production
    description: Production environment servers
    categories:
      - name: Web Servers
        description: Frontend web servers
        hosts:
          - name: Web Server 1
            description: Primary web server
            command: ssh -t jumphost@bastion "ssh -t deploy@web1 'cd /var/www && exec bash'"
          - name: Web Server 2
            description: Secondary web server
            command: ssh -t jumphost@bastion "ssh -t deploy@web2 'cd /var/www && exec bash'"
      - name: Database Servers
        description: Database servers
        hosts:
          - name: MySQL Master
            description: Primary MySQL server
            command: ssh -t jumphost@bastion "ssh -t dba@mysql-master 'exec bash'"
    hosts:
      - name: Bastion Host
        description: Jump server for production
        command: ssh jumphost@bastion

  - name: Staging
    description: Staging environment
    hosts:
      - name: Staging Server
        description: Staging environment server
        command: ssh deploy@staging

  - name: Development
    description: Development servers
    categories:
      - name: Local VMs
        description: Local virtual machines
        hosts:
          - name: Dev VM 1
            description: Development VM
            command: ssh dev@192.168.1.100
    hosts:
      - name: Dev Server
        description: Main development server
        command: ssh dev@devserver
```

### Config Schema

**Category:**
- `name`: Category name
- `description`: Description (optional)
- `icon`: Emoji icon (optional)
- `categories`: Subcategories (optional)
- `hosts`: Hosts (optional)

**Host:**
- `name`: Display name of the host
- `description`: Host description (optional)
- `command`: Single SSH command to run (for simple connections)
- `commands`: List of commands to run sequentially (for complex connections)

> **Note:** For a host you should use either `command` **or** `commands`, not both.

### Simple Connection Example

Direct connection with a single command:

```yaml
hosts:
  - name: Production Server
    description: Main production server
    command: ssh user@production.example.com
```

### Complex Connection Example (Sequential Commands)

For multi-hop connections or jump hosts:

```yaml
hosts:
  - name: Inner Server
    description: Server behind jump host
    commands:
      - ssh jumphost@bastion.example.com   # Connect to bastion first
      - sleep 2                             # Wait for connection to establish
      - ssh user@internal-server            # Then connect to internal server

  - name: Complex Setup
    description: Multi-step connection
    commands:
      - echo "Connecting to production..."
      - ssh -t jump@gateway "cd /opt/scripts && ./prepare.sh"
      - sleep 1
      - ssh -t jump@gateway "ssh app@prod-server"
```

**How Sequential Commands Work:**
- The first SSH command is detected and extended with `-tt` (for terminal allocation).
- All subsequent commands are embedded as remote commands executed within the first SSH session.
- If the last command is an SSH command, it is run via `exec` so that the user is attached directly to that session.
- Example: `["ssh host1", "sleep 2", "ssh host2"]` → `ssh -tt host1 'sleep 2; exec ssh host2'`

**Example Transformation:**
```yaml
commands:
  - ssh jumphost@bastion
  - sleep 2
  - ssh user@internal-server
```
Automatically becomes:
```bash
ssh -tt jumphost@bastion 'sleep 2; exec ssh user@internal-server'
```

### Interactive Mode (Automatic Password/Command Input)

Interactive mode lets the Go app control the SSH connection via a PTY (pseudo-terminal). This allows you to:
- Automatically enter passwords
- Send commands after the connection is established
- Finally hand control back to the user

**Special Command Prefixes:**
- `SEND:text` – Send text to the terminal (followed by Enter)
- `SENDPASS:id` – Send password from password manager (followed by Enter)
- `WAIT:N` – Wait N seconds (e.g., `WAIT:5` waits 5 seconds)
- `EXPECT:text` – Wait until the specified text appears in output (30 second timeout)
- `INTERACT` – Give control back to the user

**Example 1: Login with Password**
```yaml
hosts:
  - name: Server with Password
    description: Auto-login with password
    commands:
      - ssh user@server.com          # Start SSH
      - WAIT:2                       # Wait 2 seconds for password prompt
      - SEND:mypassword123           # Send password
      - INTERACT                     # Hand control to user
```

**Example 2: Using EXPECT for Dynamic Prompts**
```yaml
hosts:
  - name: Server with EXPECT
    description: Wait for specific prompts instead of fixed delays
    commands:
      - ssh user@server.com          # Start SSH
      - EXPECT:Password:             # Wait until "Password:" appears in output
      - SEND:mypassword123           # Send password
      - EXPECT:$                     # Wait until shell prompt appears
      - SEND:cd /opt/app             # Change directory
      - INTERACT                     # Hand control to user
```
      - WAIT:2                       # Wait for password prompt
      - SEND:mypassword123           # Send password
      - INTERACT                     # Hand control to user
```

**Example 3: Password + Automatic Commands**
```yaml
hosts:
  - name: Auto Setup Server
    description: Login and run setup commands
    commands:
      - ssh user@server.com
      - EXPECT:Password:             # Wait for password prompt (better than fixed WAIT)
      - SEND:mypassword              # Send password
      - EXPECT:$                     # Wait for shell prompt
      - SEND:cd /opt/app             # Change directory
      - SEND:./setup.sh              # Run script
      - INTERACT                     # User continues
```

**Example 4: Complex Scenario with Jump Host and Passwords**
```yaml
hosts:
  - name: Multi-Hop with Passwords
    description: Jump through multiple hosts with passwords
    commands:
      - ssh jumphost@bastion.com
      - EXPECT:Password:             # More reliable than WAIT:2
      - SEND:bastion_password
      - EXPECT:$                     # Wait for shell prompt
      - SEND:ssh user@internal-server
      - EXPECT:Password:
      - SEND:internal_password
      - INTERACT
```

**EXPECT vs WAIT:**
- `WAIT:N` – Waits for a fixed number of seconds. Simple but may wait too long or too short depending on network conditions.
- `EXPECT:text` – Waits until specific text appears in the output (max 30 seconds). More reliable for dynamic scenarios like waiting for prompts.
- Use `EXPECT` when you need to wait for specific output (like "Password:", prompt symbols "$" or "#")
- Use `WAIT` for simple delays where timing is predictable

## 🔐 Password Manager

Go-SSH includes a built-in password manager to store your passwords securely. Passwords are encrypted with AES-256-GCM and stored safely on disk.

### Using the Password Manager

Start the password manager with:

```bash
./go-ssh --passwords
```

On first run, you will be asked to create a master password. This master password protects all stored secrets.

### Menu Options

1. **Add Password** – Add a new password
   - ID: Unique identifier for the secret (e.g. `prod-db`, `staging-app`)
   - Description: Description for the secret
   - Password: The password to store

2. **List Passwords** – List stored passwords (IDs and descriptions)

3. **Remove Password** – Delete a stored password

### Using `SENDPASS` in Config

To use stored passwords in SSH connections, use the `SENDPASS:password_id` command:

```yaml
categories:
  - name: Production
    hosts:
      - name: Database Server
        description: Production database with password
        commands:
          - ssh user@db-server.com
          - SENDPASS:prod-db        # Send password from password manager
          - INTERACT
```

### Security Features

- ✅ AES-256-GCM encryption
- ✅ PBKDF2 key derivation (100,000 iterations)
- ✅ Encryption with a master password
- ✅ Only encrypted data stored on disk
- ✅ File permissions `0600` (owner read/write only)
- ✅ Passwords are decrypted in memory only when needed

### Example Workflow

1. Start the password manager:
   ```bash
   ./go-ssh --passwords
   ```

2. Add a new password:
   - ID: `prod-web`
   - Description: `Production web server password`
   - Password: `<your-secure-password>`

3. Use it in your config:
   ```yaml
   - name: Web Server
     commands:
       - ssh admin@web-server.com
       - SENDPASS:prod-web
       - INTERACT
   ```

4. Run go-ssh as usual:
   ```bash
   ./go-ssh
   ```

5. Select the host, enter your master password, and enjoy automatic login.

**Security Note:** The password manager uses AES-256 encryption and is designed to be secure, but in production environments you should prefer SSH key authentication whenever possible. Storing plain passwords directly in the YAML config (e.g. via `SEND:`) is not recommended.

## UI Preview

```
┌─────────────────────────────────────────────────────────────┐
│ 🔐 SSH Host Manager                            Hosts: 7     │
├─────────────────────────────────────────────────────────────┤
│   ▼ 🔴 Production                                           │
│     ▼ 🌐 Web Servers                                        │
│ ➤       🖥️ Web Server 1                                     │
│         🖥️ Web Server 2                                     │
│     ▶ 🗄️ Database Servers                                   │
│       🖥️ Bastion Host                                       │
│   ▶ 🟡 Staging                                              │
│   ▶ 🟢 Development                                          │
├─────────────────────────────────────────────────────────────┤
│ 🖥️ Web Server 1                                             │
│ Primary web server                                          │
│                                                             │
│ 💻 Command: ssh -t jumphost@bastion "ssh -t deploy@web1..." │
├─────────────────────────────────────────────────────────────┤
│ ↑↓/jk: Navigate  ←→/hl: Collapse/Expand  Enter: Select     │
└─────────────────────────────────────────────────────────────┘
```

## Modular Configuration with conf.d

For large configurations, you can split your config into multiple files using the `conf.d` directory.

### How It Works

All `.yaml` and `.yml` files in `~/.go-ssh/conf.d/` are automatically loaded and merged with the main config file. This allows you to organize your hosts by team, environment, or any other criteria.

### Directory Structure

```
~/.go-ssh/
├── config.yaml              # Main config (optional, can be empty)
├── conf.d/
│   ├── production.yaml      # Production servers
│   ├── staging.yaml         # Staging servers
│   ├── development.yaml     # Development servers
│   ├── team-backend.yaml    # Backend team servers
│   └── team-frontend.yaml   # Frontend team servers
└── README.md                # Auto-generated documentation
```

### Example

**Main config (`~/.go-ssh/config.yaml`):**
```yaml
# Can be empty or contain common/shared categories
categories: []
```

**conf.d/production.yaml:**
```yaml
categories:
  - name: Production
    description: Production environment
    hosts:
      - name: Web Server
        description: Production web server
        command: ssh user@web.prod.example.com
```

**conf.d/staging.yaml:**
```yaml
categories:
  - name: Staging
    description: Staging environment
    hosts:
      - name: Staging Server
        command: ssh user@staging.example.com
```

All files are automatically loaded and merged when you run `go-ssh`.

### Benefits

- 📁 **Organization**: Split large configs into logical units
- 👥 **Team collaboration**: Each team can maintain their own config file
- 🔄 **Easy updates**: Add/remove servers by adding/removing files
- 🚀 **No code changes**: Works automatically, no setup needed

## Development

To run the project:

```bash
go run main.go
```

To build:

```bash
go build -o go-ssh
```

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) – TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) – Styling and layout
- [yaml.v3](https://gopkg.in/yaml.v3) – YAML parsing

## License

MIT
