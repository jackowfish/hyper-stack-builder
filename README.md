# Hyper Stack Builder

Automated VM provisioning and image creation for Hyperstack cloud infrastructure. Built to build GPU-enabled Kubernetes node images with NVIDIA drivers and container runtime support.

## Quick Start

```bash
# Set your API key
export HYPERSTACK_API_KEY=your_key_here

# Run with config (Pass desired path to config if one doesn't exist)
go run main.go config.json
```

## Features

- Automated VM provisioning with custom scripts
- Container runtime configuration
- Snapshot and image creation

## Configuration

The tool will interactively create a config file if one doesn't exist, or you can provide your own `config.json` with VM specifications, SSH keys, and provisioning details.

To customize which scripts run or files get deployed, edit the configuration variables at the top of `main.go`.