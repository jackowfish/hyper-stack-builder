package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thundernetes/packer/kube-image/providers/hyperstack/internal/client"
	"github.com/thundernetes/packer/kube-image/providers/hyperstack/internal/config"
	"github.com/thundernetes/packer/kube-image/providers/hyperstack/internal/ssh"
	"github.com/thundernetes/packer/kube-image/providers/hyperstack/internal/types"
)

// FileDeployment represents a file to be copied to a specific destination
type FileDeployment struct {
	LocalPath  string
	RemotePath string
}

// Configuration for provisioning scripts and files
var (
	// Scripts to execute in order
	provisioningScripts = []string{
		"cleanup-nvidia-cuda.sh",
		"install-drivers.sh",
		"install-nvidia-container-toolkit.sh",
		// "install-gvisor.sh",
	}

	// Files to deploy to specific locations
	fileDeployments = []FileDeployment{
		// {
		// 	LocalPath:  "containerd-hyperstack.toml",
		// 	RemotePath: "/etc/containerd/config.toml.replacement",
		// },
		{
			LocalPath:  "runsc.toml",
			RemotePath: "/etc/containerd/runsc.toml",
		},
	}
)

func executeScripts(sshClient *ssh.Client, scripts []string, scriptDir, remoteScriptDir string) error {
	// Create remote directory
	log.Printf("Creating remote script directory: %s", remoteScriptDir)
	if err := sshClient.ExecuteCommand(fmt.Sprintf("mkdir -p %s", remoteScriptDir)); err != nil {
		return fmt.Errorf("failed to create remote script directory: %w", err)
	}

	// Copy and execute each script
	for i, script := range scripts {
		localPath := filepath.Join(scriptDir, script)
		remotePath := filepath.Join(remoteScriptDir, script)

		log.Printf("Step %d: Copying %s to VM...", i+1, script)

		// Check if local script exists
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			return fmt.Errorf("local script not found: %s", localPath)
		}

		// Copy script to VM
		if err := sshClient.CopyFile(localPath, remotePath); err != nil {
			return fmt.Errorf("failed to copy script %s: %w", script, err)
		}

		// Execute script
		log.Printf("Step %d: Executing %s...", i+1, script)
		if err := sshClient.ExecuteScript(remotePath); err != nil {
			return fmt.Errorf("failed to execute script %s: %w", script, err)
		}

		log.Printf("Step %d: Successfully executed %s", i+1, script)
	}

	return nil
}

func deployFiles(sshClient *ssh.Client, deployments []FileDeployment, filesDir string) error {
	log.Println("Deploying configuration files...")

	for _, deployment := range deployments {
		localPath := filepath.Join(filesDir, deployment.LocalPath)

		// Check if local file exists
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			return fmt.Errorf("local file not found: %s", localPath)
		}

		// Create remote directory if needed
		remoteDir := filepath.Dir(deployment.RemotePath)
		if err := sshClient.ExecuteCommand(fmt.Sprintf("sudo mkdir -p %s", remoteDir)); err != nil {
			return fmt.Errorf("failed to create remote directory %s: %w", remoteDir, err)
		}

		// Copy file to temp location first
		tempPath := fmt.Sprintf("/tmp/%s", filepath.Base(deployment.LocalPath))
		if err := sshClient.CopyFile(localPath, tempPath); err != nil {
			return fmt.Errorf("failed to copy file %s: %w", deployment.LocalPath, err)
		}

		// Move to final location with sudo
		if err := sshClient.ExecuteCommand(fmt.Sprintf("sudo mv %s %s", tempPath, deployment.RemotePath)); err != nil {
			return fmt.Errorf("failed to move file to %s: %w", deployment.RemotePath, err)
		}

		log.Printf("Successfully deployed %s to %s", deployment.LocalPath, deployment.RemotePath)
	}

	return nil
}

func executeProvisioningScripts(vmIP, privateKeyPath string) error {
	log.Println("Starting provisioning scripts execution via SSH...")

	// Create SSH client
	sshClient, err := ssh.New(privateKeyPath, "ubuntu")
	if err != nil {
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	// Connect to VM
	log.Printf("Connecting to VM at %s...", vmIP)
	if err := sshClient.Connect(vmIP); err != nil {
		return fmt.Errorf("failed to connect to VM: %w", err)
	}
	defer sshClient.Close()

	// Get directories relative to main.go
	scriptDir := filepath.Join("..", "..", "scripts")
	filesDir := filepath.Join("..", "..", "files")
	remoteScriptDir := "/tmp/provisioning-scripts"

	// Execute scripts
	if err := executeScripts(sshClient, provisioningScripts, scriptDir, remoteScriptDir); err != nil {
		return fmt.Errorf("failed to execute scripts: %w", err)
	}

	// Deploy configuration files
	if err := deployFiles(sshClient, fileDeployments, filesDir); err != nil {
		return fmt.Errorf("failed to deploy files: %w", err)
	}

	// Clean up remote scripts
	log.Println("Cleaning up remote scripts...")
	if err := sshClient.ExecuteCommand(fmt.Sprintf("rm -rf %s", remoteScriptDir)); err != nil {
		log.Printf("Warning: failed to clean up remote scripts: %v", err)
	}

	log.Println("Provisioning scripts execution completed successfully!")
	return nil
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <config-file>")
	}

	configPath := os.Args[1]

	// Check if config file exists, if not offer to create it
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("Config file '%s' not found.\n", configPath)
		fmt.Println("Would you like to create it interactively? (y/n): ")

		var response string
		fmt.Scanln(&response)

		if strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
			// Try to use API key for enhanced config generation
			apiKey := os.Getenv("HYPERSTACK_API_KEY")
			var cfg *types.Config
			if apiKey != "" {
				cfg, err = config.GenerateWithAPI(apiKey)
			} else {
				fmt.Println("HYPERSTACK_API_KEY not set, using defaults...")
				cfg, err = config.Generate()
			}

			if err != nil {
				log.Fatalf("Failed to generate config: %v", err)
			}

			if err := config.Save(cfg, configPath); err != nil {
				log.Fatalf("Failed to save config: %v", err)
			}

			fmt.Printf("Config saved to %s\n", configPath)
			fmt.Println("Please review the configuration and run the command again.")
			return
		} else {
			log.Fatal("Config file is required")
		}
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Get API key from environment
	apiKey := os.Getenv("HYPERSTACK_API_KEY")
	if apiKey == "" {
		log.Fatal("HYPERSTACK_API_KEY environment variable is required")
	}

	hyperstackClient := client.New(apiKey)

	// Make VM name unique by adding timestamp
	originalVMName := cfg.VMName
	cfg.VMName = fmt.Sprintf("%s-%d", cfg.VMName, time.Now().Unix())

	log.Printf("Creating virtual machine: %s...", cfg.VMName)
	vmResp, err := hyperstackClient.CreateVM(*cfg)
	if err != nil {
		log.Fatalf("Failed to create VM: %v", err)
	}

	// Restore original name for snapshot naming
	cfg.VMName = originalVMName

	if len(vmResp.Instances) == 0 {
		log.Fatal("No instances created")
	}

	vm := vmResp.Instances[0]
	log.Printf("Created VM: %s (ID: %d)", vm.Name, vm.ID)

	log.Println("Waiting for VM to be ready...")
	vmIP, err := hyperstackClient.WaitForVMReady(vm.ID)
	if err != nil {
		log.Fatalf("VM failed to become ready: %v", err)
	}

	// Get VM details for additional information
	log.Println("Getting VM details...")
	vmDetails, err := hyperstackClient.GetVMDetails(vm.ID)
	if err != nil {
		log.Fatalf("Failed to get VM details: %v", err)
	}

	log.Printf("VM is ready at IP: %s (FloatingIP: %s, FixedIP: %s)", vmIP, vmDetails.FloatingIP, vmDetails.FixedIP)
	log.Println("Executing provisioning scripts...")
	if err := executeProvisioningScripts(vmIP, cfg.PrivateKeyPath); err != nil {
		log.Fatalf("Provisioning failed: %v", err)
	}

	snapshotName := fmt.Sprintf("%s-snapshot-%d", cfg.VMName, time.Now().Unix())
	log.Printf("Creating snapshot: %s", snapshotName)
	snapshot, err := hyperstackClient.CreateSnapshot(vm.ID, snapshotName)
	if err != nil {
		log.Fatalf("Failed to create snapshot: %v", err)
	}

	log.Printf("Created snapshot: %s (ID: %d)", snapshot.Name, snapshot.ID)

	log.Println("Waiting for snapshot to be ready...")
	if err := hyperstackClient.WaitForSnapshotReady(snapshot.ID); err != nil {
		log.Fatalf("Snapshot failed to become ready: %v", err)
	}

	imageName := fmt.Sprintf("%s_%s", cfg.ImageName, cfg.ImageVersion)
	log.Printf("Creating image: %s", imageName)

	// Create image labels combining config tags with k8s-specific labels
	imageLabels := append([]string{}, cfg.Tags...) // Start with config tags

	// Add k8s-specific labels
	imageLabels = append(imageLabels,
		"kubernetes.io/os=linux",
		"kubernetes.io/arch=amd64",
		"nvidia.com/gpu=true",
		"nvidia.com/cuda=true",
		"container.runtime=docker",
		"image.type=kubernetes-node",
	)

	image, err := hyperstackClient.CreateImageFromSnapshot(snapshot.ID, imageName, imageLabels)
	if err != nil {
		log.Fatalf("Failed to create image: %v", err)
	}

	log.Printf("Created image: %s (ID: %d)", image.Name, image.ID)

	log.Printf("Cleaning up VM: %d", vm.ID)
	if err := hyperstackClient.DeleteVM(vm.ID); err != nil {
		log.Printf("Warning: Failed to delete VM: %v", err)
	}

	log.Println("Image creation completed successfully!")
	log.Printf("Image ID: %d", image.ID)
	log.Printf("Image Name: %s", image.Name)
}
