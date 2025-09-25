package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/thundernetes/packer/kube-image/providers/hyperstack/internal/types"
)

const (
	HyperstackAPIBase = "https://infrahub-api.nexgencloud.com/v1"
	CanadaRegionID    = 2
)

// HyperstackClient wraps the Hyperstack API client
type HyperstackClient struct {
	APIKey string
	Client *http.Client
}

// New creates a new Hyperstack API client
func New(apiKey string) *HyperstackClient {
	return &HyperstackClient{
		APIKey: apiKey,
		Client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *HyperstackClient) makeRequest(method, endpoint string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, HyperstackAPIBase+endpoint, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api_key", c.APIKey)

	return c.Client.Do(req)
}

// parseAPIResponse parses a generic Hyperstack API response
func parseAPIResponse[T any](resp *http.Response, target *T) error {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// First check the status/message wrapper
	var apiResp struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("failed to parse API response wrapper: %w", err)
	}

	if !apiResp.Status {
		return fmt.Errorf("API returned error: %s", apiResp.Message)
	}

	// Then unmarshal into the target structure
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to parse response data: %w", err)
	}

	return nil
}

// CreateVM creates a new virtual machine
func (c *HyperstackClient) CreateVM(config types.Config) (*types.VMCreateResponse, error) {
	// Create SSH security rule
	sshPort := 22
	sshRule := types.SecurityRule{
		Direction:      "ingress",
		Protocol:       "tcp",
		EtherType:      "IPv4",
		RemoteIPPrefix: "0.0.0.0/0",
		PortRangeMin:   &sshPort,
		PortRangeMax:   &sshPort,
	}

	vmReq := types.VMCreateRequest{
		Name:             config.VMName,
		ImageName:        config.BaseImageName,
		FlavorName:       config.FlavorName,
		KeyName:          config.KeypairName,
		EnvironmentName:  config.EnvironmentName,
		Count:            1,
		Labels:           config.Tags,
		AssignFloatingIP: true,
		SecurityRules:    []types.SecurityRule{sshRule},
	}

	resp, err := c.makeRequest("POST", "/core/virtual-machines", vmReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM: %w", err)
	}

	var data types.VMCreateData
	if err := parseAPIResponse(resp, &data); err != nil {
		return nil, err
	}

	return &types.VMCreateResponse{Instances: data.Instances}, nil
}

// WaitForVMReady waits for a VM to become ready and have a floating IP
func (c *HyperstackClient) WaitForVMReady(vmID int) (string, error) {
	for i := 0; i < 60; i++ { // Wait up to 10 minutes
		resp, err := c.makeRequest("GET", fmt.Sprintf("/core/virtual-machines/%d", vmID), nil)
		if err != nil {
			return "", err
		}

		var data types.VMDetailData
		if err := parseAPIResponse(resp, &data); err != nil {
			return "", err
		}

		vm := data.Instance

		// Check for ACTIVE status and floating IP attached
		if vm.Status == "ACTIVE" && vm.FloatingIP != "" && vm.FloatingIPStatus == "ATTACHED" {
			log.Printf("VM %d is ready with floating IP: %s", vmID, vm.FloatingIP)
			return vm.FloatingIP, nil
		}

		log.Printf("VM %d status: %s, floating IP: %s, status: %s, waiting...",
			vmID, vm.Status, vm.FloatingIP, vm.FloatingIPStatus)
		time.Sleep(10 * time.Second)
	}

	return "", fmt.Errorf("VM did not become ready with floating IP within timeout")
}

// GetVMDetails gets detailed information about a VM including IP address
func (c *HyperstackClient) GetVMDetails(vmID int) (*types.VMInstance, error) {
	resp, err := c.makeRequest("GET", fmt.Sprintf("/core/virtual-machines/%d", vmID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM details: %w", err)
	}

	var data types.VMDetailData
	if err := parseAPIResponse(resp, &data); err != nil {
		return nil, err
	}

	return &data.Instance, nil
}

// CreateSnapshot creates a snapshot of a VM
func (c *HyperstackClient) CreateSnapshot(vmID int, snapshotName string) (*types.Snapshot, error) {
	snapReq := types.SnapshotCreateRequest{
		Name:        snapshotName,
		Description: fmt.Sprintf("Snapshot of VM %d for image building", vmID),
	}

	resp, err := c.makeRequest("POST", fmt.Sprintf("/core/virtual-machines/%d/snapshots", vmID), snapReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create snapshot: status %d, body: %s", resp.StatusCode, string(body))
	}

	var snapshotResp types.SnapshotCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&snapshotResp); err != nil {
		return nil, fmt.Errorf("failed to decode snapshot response: %w", err)
	}

	return &snapshotResp.Snapshot, nil
}

// WaitForSnapshotReady waits for a snapshot to become ready
func (c *HyperstackClient) WaitForSnapshotReady(snapshotID int) error {
	for i := 0; i < 120; i++ { // Wait up to 20 minutes
		resp, err := c.makeRequest("GET", fmt.Sprintf("/core/snapshots/%d", snapshotID), nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		var snapshotResp types.SnapshotDetailResponse
		if err := json.NewDecoder(resp.Body).Decode(&snapshotResp); err != nil {
			return err
		}

		snapshot := snapshotResp.Snapshot
		if snapshot.Status == "SUCCESS" {
			return nil
		}

		log.Printf("Snapshot %d status: %s, waiting...", snapshotID, snapshot.Status)
		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("snapshot did not become ready within timeout")
}

// CreateImageFromSnapshot creates an image from a snapshot
func (c *HyperstackClient) CreateImageFromSnapshot(snapshotID int, imageName string, labels []string) (*types.Image, error) {
	imgReq := types.ImageCreateRequest{
		Name:   imageName,
		Labels: labels,
	}

	resp, err := c.makeRequest("POST", fmt.Sprintf("/core/snapshots/%d/image", snapshotID), imgReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create image: %w", err)
	}

	var imageResp types.ImageDetailData
	if err := parseAPIResponse(resp, &imageResp); err != nil {
		return nil, err
	}

	return &imageResp.Image, nil
}

// DeleteVM deletes a virtual machine
func (c *HyperstackClient) DeleteVM(vmID int) error {
	resp, err := c.makeRequest("DELETE", fmt.Sprintf("/core/virtual-machines/%d", vmID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete VM: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListImages lists available images
func (c *HyperstackClient) ListImages() ([]types.Image, error) {
	resp, err := c.makeRequest("GET", "/core/images", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	var data types.ImagesData
	if err := parseAPIResponse(resp, &data); err != nil {
		return nil, err
	}

	// Flatten the grouped images into a single array
	var allImages []types.Image
	for _, group := range data.Images {
		allImages = append(allImages, group.Images...)
	}

	return allImages, nil
}

// ListRegions lists available regions
func (c *HyperstackClient) ListRegions() ([]types.Region, error) {
	resp, err := c.makeRequest("GET", "/core/regions", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list regions: %w", err)
	}

	var data types.RegionsData
	if err := parseAPIResponse(resp, &data); err != nil {
		return nil, err
	}

	return data.Regions, nil
}

// ListFlavors lists available VM flavors
func (c *HyperstackClient) ListFlavors() ([]types.Flavor, error) {
	resp, err := c.makeRequest("GET", "/core/flavors", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list flavors: %w", err)
	}

	var data types.FlavorsData
	if err := parseAPIResponse(resp, &data); err != nil {
		return nil, err
	}

	// Flatten the grouped flavors into a single array
	var allFlavors []types.Flavor
	for _, group := range data.Data {
		allFlavors = append(allFlavors, group.Flavors...)
	}

	return allFlavors, nil
}

// ListKeypairs lists available SSH keypairs
func (c *HyperstackClient) ListKeypairs() ([]types.Keypair, error) {
	resp, err := c.makeRequest("GET", "/core/keypairs", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list keypairs: %w", err)
	}

	var data types.KeypairsData
	if err := parseAPIResponse(resp, &data); err != nil {
		return nil, err
	}

	return data.Keypairs, nil
}

// ListEnvironments lists available environments
func (c *HyperstackClient) ListEnvironments() ([]types.Environment, error) {
	resp, err := c.makeRequest("GET", "/core/environments", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}

	var data types.EnvironmentsData
	if err := parseAPIResponse(resp, &data); err != nil {
		return nil, err
	}

	return data.Environments, nil
}
