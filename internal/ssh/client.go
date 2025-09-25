package ssh

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Client wraps SSH connectivity
type Client struct {
	config *ssh.ClientConfig
	client *ssh.Client
}

// New creates a new SSH client with private key authentication
func New(privateKeyPath, username string) (*Client, error) {
	// Expand tilde in path
	if strings.HasPrefix(privateKeyPath, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		privateKeyPath = filepath.Join(homeDir, privateKeyPath[1:])
	}

	// Read private key
	key, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	// Parse private key
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Note: In production, use proper host key verification
		Timeout:         30 * time.Second,
	}

	return &Client{config: config}, nil
}

// Connect establishes SSH connection to the remote host
func (c *Client) Connect(host string) error {
	var err error
	// Try connecting with retries for up to 5 minutes
	for attempt := 0; attempt < 30; attempt++ {
		c.client, err = ssh.Dial("tcp", host+":22", c.config)
		if err == nil {
			log.Printf("SSH connection established to %s", host)
			return nil
		}
		
		log.Printf("SSH connection attempt %d failed: %v, retrying in 10s...", attempt+1, err)
		time.Sleep(10 * time.Second)
	}
	
	return fmt.Errorf("failed to connect after 30 attempts: %w", err)
}

// Close closes the SSH connection
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// CopyFile copies a local file to the remote host via SCP
func (c *Client) CopyFile(localPath, remotePath string) error {
	if c.client == nil {
		return fmt.Errorf("SSH connection not established")
	}

	// Read local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	stat, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}

	// Create SCP session
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Set up SCP command
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()
		
		fmt.Fprintf(w, "C0644 %d %s\n", stat.Size(), filepath.Base(remotePath))
		io.Copy(w, localFile)
		fmt.Fprint(w, "\x00")
	}()

	// Execute SCP command
	cmd := fmt.Sprintf("scp -t %s", remotePath)
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to execute SCP: %w", err)
	}

	log.Printf("File copied: %s -> %s", localPath, remotePath)
	return nil
}

// ExecuteCommand executes a command on the remote host
func (c *Client) ExecuteCommand(command string) error {
	if c.client == nil {
		return fmt.Errorf("SSH connection not established")
	}

	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Set up stdout/stderr capture
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	log.Printf("Executing command: %s", command)
	if err := session.Run(command); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// ExecuteScript executes a script with proper permissions
func (c *Client) ExecuteScript(scriptPath string) error {
	// Make script executable
	if err := c.ExecuteCommand(fmt.Sprintf("chmod +x %s", scriptPath)); err != nil {
		return fmt.Errorf("failed to make script executable: %w", err)
	}

	// Execute script
	if err := c.ExecuteCommand(scriptPath); err != nil {
		return fmt.Errorf("failed to execute script: %w", err)
	}

	return nil
}