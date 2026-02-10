package password

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"
)

const (
	// PBKDF2 parameters
	saltSize   = 32
	iterations = 100000
	keySize    = 32 // AES-256
)

// PasswordEntry represents a stored password
type PasswordEntry struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Password    string `json:"password"` // Encrypted
}

// PasswordStore manages encrypted passwords
type PasswordStore struct {
	filePath string
	entries  map[string]*PasswordEntry
}

// NewPasswordStore creates a new password store
func NewPasswordStore() *PasswordStore {
	homeDir, _ := os.UserHomeDir()
	filePath := filepath.Join(homeDir, ".go-ssh", "passwords.enc")

	return &PasswordStore{
		filePath: filePath,
		entries:  make(map[string]*PasswordEntry),
	}
}

// GetStorePath returns the password store file path
func (ps *PasswordStore) GetStorePath() string {
	return ps.filePath
}

// StoreExists checks if the password store file exists
func (ps *PasswordStore) StoreExists() bool {
	_, err := os.Stat(ps.filePath)
	return err == nil
}

// deriveMasterKey derives an encryption key from master password
func deriveMasterKey(masterPassword string, salt []byte) []byte {
	return pbkdf2.Key([]byte(masterPassword), salt, iterations, keySize, sha256.New)
}

// encrypt encrypts data using AES-GCM
func encrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decrypt decrypts data using AES-GCM
func decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// PromptMasterPassword prompts user for master password securely
func PromptMasterPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // New line after password input
	if err != nil {
		return "", err
	}
	return string(password), nil
}

// Initialize creates a new encrypted password store
func (ps *PasswordStore) Initialize(masterPassword string) error {
	// Generate random salt
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	// Create empty store
	ps.entries = make(map[string]*PasswordEntry)

	// Save with empty entries
	if err := ps.Save(masterPassword, salt); err != nil {
		return fmt.Errorf("failed to save initial store: %w", err)
	}

	return nil
}

// Load loads and decrypts the password store
func (ps *PasswordStore) Load(masterPassword string) error {
	// Read encrypted file
	data, err := os.ReadFile(ps.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, initialize empty store
			ps.entries = make(map[string]*PasswordEntry)
			return nil
		}
		return fmt.Errorf("failed to read password store: %w", err)
	}

	// Extract salt (first 32 bytes)
	if len(data) < saltSize {
		return fmt.Errorf("invalid password store format")
	}

	salt := data[:saltSize]
	encryptedData := data[saltSize:]

	// Derive key from master password
	key := deriveMasterKey(masterPassword, salt)

	// Decrypt
	decryptedData, err := decrypt(encryptedData, key)
	if err != nil {
		return fmt.Errorf("failed to decrypt (wrong password?): %w", err)
	}

	// Parse JSON
	var entries []*PasswordEntry
	if err := json.Unmarshal(decryptedData, &entries); err != nil {
		return fmt.Errorf("failed to parse password store: %w", err)
	}

	// Build map
	ps.entries = make(map[string]*PasswordEntry)
	for _, entry := range entries {
		// Decrypt individual passwords
		passwordBytes, err := base64.StdEncoding.DecodeString(entry.Password)
		if err != nil {
			return fmt.Errorf("failed to decode password for %s: %w", entry.ID, err)
		}

		decryptedPassword, err := decrypt(passwordBytes, key)
		if err != nil {
			return fmt.Errorf("failed to decrypt password for %s: %w", entry.ID, err)
		}

		entry.Password = string(decryptedPassword)
		ps.entries[entry.ID] = entry
	}

	return nil
}

// Save encrypts and saves the password store
func (ps *PasswordStore) Save(masterPassword string, salt []byte) error {
	// If no salt provided, read from existing file or generate new
	if salt == nil {
		if ps.StoreExists() {
			data, err := os.ReadFile(ps.filePath)
			if err == nil && len(data) >= saltSize {
				salt = data[:saltSize]
			}
		}
		if salt == nil {
			salt = make([]byte, saltSize)
			if _, err := rand.Read(salt); err != nil {
				return fmt.Errorf("failed to generate salt: %w", err)
			}
		}
	}

	// Derive key
	key := deriveMasterKey(masterPassword, salt)

	// Encrypt individual passwords and prepare for JSON
	entriesToSave := make([]*PasswordEntry, 0, len(ps.entries))
	for _, entry := range ps.entries {
		// Encrypt password
		encryptedPassword, err := encrypt([]byte(entry.Password), key)
		if err != nil {
			return fmt.Errorf("failed to encrypt password for %s: %w", entry.ID, err)
		}

		entriesToSave = append(entriesToSave, &PasswordEntry{
			ID:          entry.ID,
			Description: entry.Description,
			Password:    base64.StdEncoding.EncodeToString(encryptedPassword),
		})
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(entriesToSave, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal passwords: %w", err)
	}

	// Encrypt JSON data
	encryptedData, err := encrypt(jsonData, key)
	if err != nil {
		return fmt.Errorf("failed to encrypt data: %w", err)
	}

	// Combine salt + encrypted data
	finalData := append(salt, encryptedData...)

	// Ensure directory exists
	dir := filepath.Dir(ps.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to file with restricted permissions
	if err := os.WriteFile(ps.filePath, finalData, 0600); err != nil {
		return fmt.Errorf("failed to write password store: %w", err)
	}

	return nil
}

// Add adds a new password entry
func (ps *PasswordStore) Add(id, description, password string) error {
	if _, exists := ps.entries[id]; exists {
		return fmt.Errorf("password with ID '%s' already exists", id)
	}

	ps.entries[id] = &PasswordEntry{
		ID:          id,
		Description: description,
		Password:    password,
	}

	return nil
}

// Get retrieves a password by ID
func (ps *PasswordStore) Get(id string) (string, error) {
	entry, exists := ps.entries[id]
	if !exists {
		return "", fmt.Errorf("password with ID '%s' not found", id)
	}

	return entry.Password, nil
}

// GetEntry retrieves a full password entry by ID
func (ps *PasswordStore) GetEntry(id string) (*PasswordEntry, error) {
	entry, exists := ps.entries[id]
	if !exists {
		return nil, fmt.Errorf("password with ID '%s' not found", id)
	}

	return entry, nil
}

// Remove removes a password entry
func (ps *PasswordStore) Remove(id string) error {
	if _, exists := ps.entries[id]; !exists {
		return fmt.Errorf("password with ID '%s' not found", id)
	}

	delete(ps.entries, id)
	return nil
}

// List returns all password entries (without actual passwords)
func (ps *PasswordStore) List() []*PasswordEntry {
	entries := make([]*PasswordEntry, 0, len(ps.entries))
	for _, entry := range ps.entries {
		entries = append(entries, &PasswordEntry{
			ID:          entry.ID,
			Description: entry.Description,
			Password:    "***", // Don't expose password
		})
	}
	return entries
}

// Count returns the number of stored passwords
func (ps *PasswordStore) Count() int {
	return len(ps.entries)
}

// ChangeMasterPassword changes the master password
func (ps *PasswordStore) ChangeMasterPassword(oldPassword, newPassword string) error {
	// Verify old password by trying to load with it
	data, err := os.ReadFile(ps.filePath)
	if err != nil {
		return fmt.Errorf("failed to read password store: %w", err)
	}

	if len(data) < saltSize {
		return fmt.Errorf("invalid password store format")
	}

	oldSalt := data[:saltSize]
	encryptedData := data[saltSize:]

	// Verify old password
	oldKey := deriveMasterKey(oldPassword, oldSalt)
	decryptedData, err := decrypt(encryptedData, oldKey)
	if err != nil {
		return fmt.Errorf("incorrect old password")
	}

	// Parse entries
	var entries []*PasswordEntry
	if err := json.Unmarshal(decryptedData, &entries); err != nil {
		return fmt.Errorf("failed to parse password store: %w", err)
	}

	// Decrypt individual passwords with old key
	ps.entries = make(map[string]*PasswordEntry)
	for _, entry := range entries {
		passwordBytes, err := base64.StdEncoding.DecodeString(entry.Password)
		if err != nil {
			return fmt.Errorf("failed to decode password for %s: %w", entry.ID, err)
		}

		decryptedPassword, err := decrypt(passwordBytes, oldKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt password for %s: %w", entry.ID, err)
		}

		entry.Password = string(decryptedPassword)
		ps.entries[entry.ID] = entry
	}

	// Generate new salt for new password
	newSalt := make([]byte, saltSize)
	if _, err := rand.Read(newSalt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	// Save with new password
	if err := ps.Save(newPassword, newSalt); err != nil {
		return fmt.Errorf("failed to save with new password: %w", err)
	}

	return nil
}
