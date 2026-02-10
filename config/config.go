package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Host represents an SSH host configuration
type Host struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	Command     string   `yaml:"command,omitempty"`  // Single command (legacy)
	Commands    []string `yaml:"commands,omitempty"` // Multiple commands for complex connections
}

// GetCommands returns the command list for the host
// If Commands is set, returns it; otherwise wraps Command in a slice
func (h *Host) GetCommands() []string {
	if len(h.Commands) > 0 {
		return h.Commands
	}
	if h.Command != "" {
		return []string{h.Command}
	}
	return nil
}

// Category represents a category that can contain hosts and subcategories
type Category struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description,omitempty"`
	Categories  []Category `yaml:"categories,omitempty"`
	Hosts       []Host     `yaml:"hosts,omitempty"`
}

// Config represents the application configuration
type Config struct {
	Categories []Category `yaml:"categories"`
}

// GetConfigDir returns the config directory path
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".go-ssh"), nil
}

// GetConfigPath returns the config file path
func GetConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.yaml"), nil
}

// EnsureConfigDir creates the config directory if it doesn't exist
func EnsureConfigDir() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	return os.MkdirAll(configDir, 0755)
}

// LoadConfig loads the configuration from the YAML file
func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		return createDefaultConfig()
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	return &config, nil
}

// SaveConfig saves the configuration to the YAML file
func SaveConfig(config *Config) error {
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}

// createDefaultConfig creates a default configuration file
func createDefaultConfig() (*Config, error) {
	config := &Config{
		Categories: []Category{
			{
				Name:        "Production",
				Description: "Production environment servers",
				Categories: []Category{
					{
						Name:        "Web Servers",
						Description: "Frontend web servers",
						Hosts: []Host{
							{
								Name:        "Web Server 1",
								Description: "Primary web server",
								Command:     `ssh -t jumphost@bastion "ssh -t deploy@web1 'cd /var/www && exec bash'"`,
							},
							{
								Name:        "Web Server 2",
								Description: "Secondary web server",
								Command:     `ssh -t jumphost@bastion "ssh -t deploy@web2 'cd /var/www && exec bash'"`,
							},
						},
					},
					{
						Name:        "Database Servers",
						Description: "Database servers",
						Hosts: []Host{
							{
								Name:        "MySQL Master",
								Description: "Primary MySQL server",
								Command:     `ssh -t jumphost@bastion "ssh -t dba@mysql-master 'exec bash'"`,
							},
						},
					},
				},
				Hosts: []Host{
					{
						Name:        "Bastion Host",
						Description: "Jump server for production",
						Command:     "ssh jumphost@bastion",
					},
				},
			},
			{
				Name:        "Staging",
				Description: "Staging environment",
				Hosts: []Host{
					{
						Name:        "Staging Server",
						Description: "Staging environment server",
						Command:     "ssh deploy@staging",
					},
				},
			},
			{
				Name:        "Development",
				Description: "Development servers",
				Categories: []Category{
					{
						Name:        "Local VMs",
						Description: "Local virtual machines",
						Hosts: []Host{
							{
								Name:        "Dev VM 1",
								Description: "Development VM",
								Command:     "ssh dev@192.168.1.100",
							},
						},
					},
				},
				Hosts: []Host{
					{
						Name:        "Dev Server",
						Description: "Main development server",
						Command:     "ssh dev@devserver",
					},
				},
			},
		},
	}

	if err := SaveConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// TreeNode represents a node in the tree (can be category or host)
type TreeNode struct {
	Name        string
	Description string
	IsCategory  bool
	IsExpanded  bool
	Level       int
	Command     string      // Only for hosts (single command)
	Commands    []string    // Only for hosts (multiple commands)
	Children    []*TreeNode // Only for categories
	Parent      *TreeNode
}

// ToHost converts a TreeNode to a Host (only for host nodes)
func (tn *TreeNode) ToHost() *Host {
	if tn.IsCategory {
		return nil
	}
	return &Host{
		Name:        tn.Name,
		Description: tn.Description,
		Command:     tn.Command,
		Commands:    tn.Commands,
	}
}

// BuildTree builds a tree structure from the config
func BuildTree(cfg *Config) []*TreeNode {
	var nodes []*TreeNode
	for i := range cfg.Categories {
		node := buildCategoryNode(&cfg.Categories[i], 0, nil)
		nodes = append(nodes, node)
	}
	return nodes
}

func buildCategoryNode(cat *Category, level int, parent *TreeNode) *TreeNode {
	node := &TreeNode{
		Name:        cat.Name,
		Description: cat.Description,
		IsCategory:  true,
		IsExpanded:  false,
		Level:       level,
		Parent:      parent,
	}

	// Add subcategories
	for i := range cat.Categories {
		child := buildCategoryNode(&cat.Categories[i], level+1, node)
		node.Children = append(node.Children, child)
	}

	// Add hosts
	for _, host := range cat.Hosts {
		hostNode := &TreeNode{
			Name:        host.Name,
			Description: host.Description,
			IsCategory:  false,
			Level:       level + 1,
			Command:     host.Command,
			Commands:    host.Commands,
			Parent:      node,
		}
		node.Children = append(node.Children, hostNode)
	}

	return node
}

// GetVisibleNodes returns all visible nodes based on expanded state
func GetVisibleNodes(roots []*TreeNode) []*TreeNode {
	var visible []*TreeNode
	for _, root := range roots {
		visible = append(visible, getVisibleNodesRecursive(root)...)
	}
	return visible
}

func getVisibleNodesRecursive(node *TreeNode) []*TreeNode {
	nodes := []*TreeNode{node}
	if node.IsCategory && node.IsExpanded {
		for _, child := range node.Children {
			nodes = append(nodes, getVisibleNodesRecursive(child)...)
		}
	}
	return nodes
}
