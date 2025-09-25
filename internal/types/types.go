package types

// Config holds the configuration for building Hyperstack images
type Config struct {
	Region          string   `json:"region"`
	ImageName       string   `json:"image_name"`
	ImageVersion    string   `json:"image_version"`
	BaseImageName   string   `json:"base_image_name"`
	VMName          string   `json:"vm_name"`
	FlavorName      string   `json:"flavor_name"`
	KeypairName     string   `json:"keypair_name"`
	PrivateKeyPath  string   `json:"private_key_path"`
	EnvironmentName string   `json:"environment_name"`
	Tags            []string `json:"tags"`
}

// SecurityRule represents a security rule for VM creation
type SecurityRule struct {
	Direction       string `json:"direction"`
	Protocol        string `json:"protocol"`
	EtherType       string `json:"ethertype"`
	RemoteIPPrefix  string `json:"remote_ip_prefix"`
	PortRangeMin    *int   `json:"port_range_min,omitempty"`
	PortRangeMax    *int   `json:"port_range_max,omitempty"`
}

// VMCreateRequest represents a request to create a virtual machine
type VMCreateRequest struct {
	Name                   string          `json:"name"`
	ImageName              string          `json:"image_name"`
	FlavorName             string          `json:"flavor_name"`
	KeyName                string          `json:"key_name"`
	EnvironmentName        string          `json:"environment_name"`
	Count                  int             `json:"count"`
	Labels                 []string        `json:"labels"`
	AssignFloatingIP       bool            `json:"assign_floating_ip"`
	EnablePortRandomization *bool          `json:"enable_port_randomization,omitempty"`
	SecurityRules          []SecurityRule  `json:"security_rules,omitempty"`
}

// VMInstance represents a virtual machine instance
type VMInstance struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	FixedIP          string `json:"fixed_ip"`
	FloatingIP       string `json:"floating_ip"`
	FloatingIPStatus string `json:"floating_ip_status"`
	Flavor    VMFlavor `json:"flavor"`
	Image     VMImage  `json:"image"`
}

// VMFlavor represents VM flavor information
type VMFlavor struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// VMImage represents VM image information
type VMImage struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// VMCreateResponse represents the response from creating VMs
type VMCreateResponse struct {
	Instances []VMInstance `json:"instances"`
}

// SnapshotCreateRequest represents a request to create a snapshot
type SnapshotCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Snapshot represents a VM snapshot
type Snapshot struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	VMID          int    `json:"vm_id"`
	RegionID      int    `json:"region_id"`
	Status        string `json:"status"`
	IsImage       bool   `json:"is_image"`
	Size          int    `json:"size"`
	HasFloatingIP bool   `json:"has_floating_ip"`
	Labels        []any  `json:"labels"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type SnapshotCreateResponse struct {
	Status   bool     `json:"status"`
	Message  string   `json:"message"`
	Snapshot Snapshot `json:"snapshot"`
}

type ImageCreateData struct {
	Image Image `json:"image"`
}

type SnapshotDetailResponse struct {
	Status   int      `json:"status"`
	Message  string   `json:"message"`
	Snapshot Snapshot `json:"snapshot"`
}

type ImageDetailData struct {
	Status  bool  `json:"status"`
	Message string `json:"message"`
	Image   Image `json:"image"`
}

// ImageCreateRequest represents a request to create an image from snapshot
type ImageCreateRequest struct {
	Name   string   `json:"name"`
	Labels []string `json:"labels,omitempty"`
}

// ImageLabel represents a label on an image
type ImageLabel struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// Image represents a Hyperstack image
type Image struct {
	ID         int          `json:"id"`
	Name       string       `json:"name"`
	RegionName string       `json:"region_name"`
	Type       string       `json:"type"`
	Version    string       `json:"version"`
	Size       int64        `json:"size"`
	IsPublic   bool         `json:"is_public"`
	Labels     []ImageLabel `json:"labels"`
}

// ImageGroup represents grouped images by region/type
type ImageGroup struct {
	RegionName string  `json:"region_name"`
	Type       string  `json:"type"`
	Images     []Image `json:"images"`
}

// APIResponse represents the standard Hyperstack API response wrapper
type APIResponse[T any] struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    T      `json:"-"` // Will be unmarshaled separately
}

// Specific response data structures
type ImagesData struct {
	Images []ImageGroup `json:"images"`
}

type RegionsData struct {
	Regions []Region `json:"regions"`
}

type FlavorsData struct {
	Data []FlavorGroup `json:"data"`
}

type KeypairsData struct {
	Keypairs []Keypair `json:"keypairs"`
}

type EnvironmentsData struct {
	Environments []Environment `json:"environments"`
}

type VMCreateData struct {
	Instances []VMInstance `json:"instances"`
}

type VMDetailData struct {
	Instance VMInstance `json:"instance"`
}

// Region represents a Hyperstack region
type Region struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Flavor represents a VM flavor/instance type
type Flavor struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	RegionName string  `json:"region_name"`
	CPU        int     `json:"cpu"`
	RAM        float64 `json:"ram"`
	Disk       int     `json:"disk"`
	GPU        string  `json:"gpu"`
	GPUCount   int     `json:"gpu_count"`
}

// FlavorGroup represents grouped flavors by GPU type and region
type FlavorGroup struct {
	GPU        string    `json:"gpu"`
	RegionName string    `json:"region_name"`
	Flavors    []Flavor  `json:"flavors"`
}

// Environment represents a Hyperstack environment
type Environment struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Keypair represents an SSH keypair
type Keypair struct {
	ID          int         `json:"id"`
	Name        string      `json:"name"`
	Environment Environment `json:"environment"`
	Fingerprint string      `json:"fingerprint"`
}