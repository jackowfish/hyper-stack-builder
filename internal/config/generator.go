package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/thundernetes/packer/kube-image/providers/hyperstack/internal/client"
	"github.com/thundernetes/packer/kube-image/providers/hyperstack/internal/types"
)

// PromptUser prompts the user for input with an optional default value
func PromptUser(prompt string, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" && defaultValue != "" {
		return defaultValue
	}
	return input
}

// GenerateWithAPI creates a new configuration interactively using API data
func GenerateWithAPI(apiKey string) (*types.Config, error) {
	fmt.Println("=== Hyperstack Image Builder Configuration ===")
	fmt.Println("This will generate a config.json file for building Kubernetes GPU images.")
	fmt.Println("Fetching available options from Hyperstack API...")
	fmt.Println()

	hyperstackClient := client.New(apiKey)
	config := &types.Config{}

	// Fetch available resources
	images, err := hyperstackClient.ListImages()
	if err != nil {
		fmt.Printf("Warning: Could not fetch images: %v\n", err)
		fmt.Println("Using default values...")
	}

	regions, err := hyperstackClient.ListRegions()
	if err != nil {
		fmt.Printf("Warning: Could not fetch regions: %v\n", err)
	}

	flavors, err := hyperstackClient.ListFlavors()
	if err != nil {
		fmt.Printf("Warning: Could not fetch flavors: %v\n", err)
	}

	keypairs, err := hyperstackClient.ListKeypairs()
	if err != nil {
		fmt.Printf("Warning: Could not fetch keypairs: %v\n", err)
	}

	environments, err := hyperstackClient.ListEnvironments()
	if err != nil {
		fmt.Printf("Warning: Could not fetch environments: %v\n", err)
	}

	// Show available regions and let user select
	var selectedRegion string
	if len(regions) > 0 {
		fmt.Println("Available regions:")
		for i, region := range regions {
			fmt.Printf("  %d. %s (ID: %d)\n", i+1, region.Name, region.ID)
		}
		
		// Default to Canada for the original requirements
		defaultChoice := "2" // CANADA-1
		for i, region := range regions {
			if region.Name == "CANADA-1" {
				defaultChoice = fmt.Sprintf("%d", i+1)
				break
			}
		}
		
		choice := PromptUser(fmt.Sprintf("Select region (1-%d)", len(regions)), defaultChoice)
		if num, err := strconv.Atoi(choice); err == nil && num > 0 && num <= len(regions) {
			selectedRegion = regions[num-1].Name
		} else {
			selectedRegion = "CANADA-1" // fallback
		}
		fmt.Printf("Selected region: %s\n\n", selectedRegion)
	} else {
		selectedRegion = "CANADA-1"
	}

	// Set the selected region in config
	config.Region = selectedRegion

	// Image configuration
	config.ImageName = PromptUser("Output image name", "kubernetes_gpu_cuda")
	config.ImageVersion = PromptUser("Output image version", fmt.Sprintf("202508.%02d.0", time.Now().Day()))

	// Show available base images filtered by selected region and k8s label
	if len(images) > 0 {
		fmt.Printf("Available base images in %s (k8s-compatible images):\n", selectedRegion)
		k8sImages := []types.Image{}
		for _, img := range images {
			// Filter by region and k8s label
			hasK8sLabel := false
			for _, labelObj := range img.Labels {
				if strings.Contains(strings.ToLower(labelObj.Label), "k8s") || 
				   strings.Contains(strings.ToLower(labelObj.Label), "kubernetes") {
					hasK8sLabel = true
					break
				}
			}
			
			if img.RegionName == selectedRegion && hasK8sLabel {
				k8sImages = append(k8sImages, img)
			}
		}
		
		// If no k8s labeled images found, fall back to Ubuntu/Docker images as before
		if len(k8sImages) == 0 {
			fmt.Printf("No k8s-labeled images found, showing Ubuntu/Docker images:\n")
			for _, img := range images {
				if img.RegionName == selectedRegion &&
				   strings.Contains(strings.ToLower(img.Name), "ubuntu") && 
				   strings.Contains(strings.ToLower(img.Name), "docker") {
					k8sImages = append(k8sImages, img)
				}
			}
		}
		
		ubuntuImages := k8sImages // Rename for consistency with rest of code
		
		for i, img := range ubuntuImages {
			if i >= 10 { // Limit display to first 10
				fmt.Println("  ... (showing first 10)")
				break
			}
			fmt.Printf("  %d. %s (Size: %.1fGB, Public: %v)\n", i+1, img.Name, float64(img.Size)/1024/1024/1024, img.IsPublic)
		}
		
		if len(ubuntuImages) > 0 {
			choice := PromptUser(fmt.Sprintf("Select base image (1-%d) or enter custom name", len(ubuntuImages)), "1")
			if num, err := strconv.Atoi(choice); err == nil && num > 0 && num <= len(ubuntuImages) {
				config.BaseImageName = ubuntuImages[num-1].Name
			} else {
				config.BaseImageName = choice
			}
		} else {
			config.BaseImageName = PromptUser("Base image name", "Ubuntu Server 22.04 LTS R535 CUDA 12.2 with Docker")
		}
	} else {
		config.BaseImageName = PromptUser("Base image name", "Ubuntu Server 22.04 LTS R535 CUDA 12.2 with Docker")
	}

	// VM configuration
	config.VMName = PromptUser("Temporary VM name", "thunder-build-vm")

	// Show available flavors (GPU ones first) filtered by selected region
	if len(flavors) > 0 {
		fmt.Printf("\nAvailable VM flavors in %s (GPU instances):\n", selectedRegion)
		gpuFlavors := []types.Flavor{}
		for _, flavor := range flavors {
			if flavor.GPUCount > 0 && flavor.RegionName == selectedRegion {
				gpuFlavors = append(gpuFlavors, flavor)
			}
		}
		
		for i, flavor := range gpuFlavors {
			if i >= 10 { // Limit display to first 10
				fmt.Println("  ... (showing first 10 GPU flavors)")
				break
			}
			fmt.Printf("  %d. %s (CPU: %d, RAM: %.0fGB, GPU: %d %s)\n", 
				i+1, flavor.Name, flavor.CPU, flavor.RAM, flavor.GPUCount, flavor.GPU)
		}
		
		if len(gpuFlavors) > 0 {
			choice := PromptUser(fmt.Sprintf("Select flavor (1-%d) or enter custom name", len(gpuFlavors)), "1")
			if num, err := strconv.Atoi(choice); err == nil && num > 0 && num <= len(gpuFlavors) {
				config.FlavorName = gpuFlavors[num-1].Name
			} else {
				config.FlavorName = choice
			}
		} else {
			config.FlavorName = PromptUser("VM flavor (GPU instance type)", "n1-A100x1")
		}
	} else {
		config.FlavorName = PromptUser("VM flavor (GPU instance type)", "n1-A100x1")
	}

	// Show available keypairs
	if len(keypairs) > 0 {
		fmt.Println("\nAvailable SSH keypairs:")
		for i, kp := range keypairs {
			if i >= 10 { // Limit display to first 10
				fmt.Println("  ... (showing first 10)")
				break
			}
			fmt.Printf("  %d. %s (Environment: %s)\n", i+1, kp.Name, kp.Environment.Name)
		}
		
		choice := PromptUser(fmt.Sprintf("Select keypair (1-%d) or enter custom name", len(keypairs)), "")
		if choice != "" {
			if num, err := strconv.Atoi(choice); err == nil && num > 0 && num <= len(keypairs) {
				config.KeypairName = keypairs[num-1].Name
			} else {
				config.KeypairName = choice
			}
		}
	} else {
		config.KeypairName = PromptUser("SSH keypair name", "")
	}

	// Private key path for SSH access
	config.PrivateKeyPath = PromptUser("Private key path for SSH access", "~/.ssh/id_rsa")

	// Show available environments filtered by selected region
	if len(environments) > 0 {
		fmt.Printf("\nAvailable environments in %s:\n", selectedRegion)
		regionEnvironments := []types.Environment{}
		for _, env := range environments {
			// Filter environments that match the selected region
			if strings.Contains(env.Name, selectedRegion) {
				regionEnvironments = append(regionEnvironments, env)
			}
		}
		
		if len(regionEnvironments) > 0 {
			for i, env := range regionEnvironments {
				fmt.Printf("  %d. %s (ID: %d)\n", i+1, env.Name, env.ID)
			}
			
			choice := PromptUser(fmt.Sprintf("Select environment (1-%d) or enter custom name", len(regionEnvironments)), "1")
			if num, err := strconv.Atoi(choice); err == nil && num > 0 && num <= len(regionEnvironments) {
				config.EnvironmentName = regionEnvironments[num-1].Name
			} else {
				config.EnvironmentName = choice
			}
		} else {
			fmt.Println("No environments found for this region, using default pattern")
			config.EnvironmentName = fmt.Sprintf("default-%s", selectedRegion)
		}
	} else {
		config.EnvironmentName = fmt.Sprintf("default-%s", selectedRegion)
	}

	// Tags - simple labels, automatically include k8s
	fmt.Println("\nConfigure tags (simple labels):")
	config.Tags = []string{"k8s"}
	fmt.Println("Added: k8s")

	// Allow additional custom tags
	fmt.Println("\nAdd custom labels (just enter label names, empty line to finish):")
	for {
		input := PromptUser("Custom label", "")
		if input == "" {
			break
		}
		
		config.Tags = append(config.Tags, input)
		fmt.Printf("Added: %s\n", input)
	}

	return config, nil
}

// Generate creates a new configuration interactively (fallback without API)
func Generate() (*types.Config, error) {
	fmt.Println("=== Hyperstack Image Builder Configuration ===")
	fmt.Println("This will generate a config.json file for building Kubernetes GPU images.")
	fmt.Println("(Using default values - API key not available for fetching options)")
	fmt.Println()

	config := &types.Config{}

	// Image configuration
	config.ImageName = PromptUser("Image name", "kubernetes_gpu_cuda")
	config.ImageVersion = PromptUser("Image version", fmt.Sprintf("202508.%02d.0", time.Now().Day()))
	config.BaseImageName = PromptUser("Base image name", "Ubuntu Server 22.04 LTS R535 CUDA 12.2 with Docker")

	// VM configuration
	config.VMName = PromptUser("Temporary VM name", "thunder-build-vm")
	config.FlavorName = PromptUser("VM flavor (GPU instance type)", "n1-A100x1")
	config.KeypairName = PromptUser("SSH keypair name", "")
	config.PrivateKeyPath = PromptUser("Private key path for SSH access", "~/.ssh/id_rsa")
	config.EnvironmentName = PromptUser("Environment name", "default")

	// Tags - simple labels, automatically include k8s
	fmt.Println("\nConfigure tags (simple labels):")
	config.Tags = []string{"k8s"}
	fmt.Println("Added: k8s")

	// Allow additional custom tags
	fmt.Println("\nAdd custom labels (just enter label names, empty line to finish):")
	for {
		input := PromptUser("Custom label", "")
		if input == "" {
			break
		}
		
		config.Tags = append(config.Tags, input)
		fmt.Printf("Added: %s\n", input)
	}

	return config, nil
}

// Save writes the configuration to a file
func Save(config *types.Config, filename string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// Load reads the configuration from a file
func Load(filename string) (*types.Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config types.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Set defaults if not specified
	if config.FlavorName == "" {
		config.FlavorName = "n1-A100x1"
	}
	if config.BaseImageName == "" {
		config.BaseImageName = "Ubuntu Server 22.04 LTS R535 CUDA 12.2 with Docker"
	}
	if config.Tags == nil {
		config.Tags = []string{"k8s"}
	}

	return &config, nil
}