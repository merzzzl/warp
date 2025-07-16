# WARP

WARP provides an advanced solution for tunneling network traffic through various protocols: SSH, SOCKS5, and WireGuard. This tool ensures secure and efficient network data routing with a convenient terminal user interface and real-time monitoring.

![WARP run with TUI mode](README.png)

## Table of Contents

- [Introduction](#introduction)
- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
  - [Command Line Options](#command-line-options)
  - [Configuration File](#configuration-file)
- [Configuration Examples](#configuration-examples)
  - [SSH Tunnel](#ssh-tunnel)
  - [SOCKS5 Proxy](#socks5-proxy)
  - [WireGuard VPN](#wireguard-vpn)
- [Monitoring](#monitoring)
- [License](#license)

## Introduction

WARP offers a unified interface for working with different types of tunnels. It creates a virtual TUN interface, configures DNS and routing, and directs traffic through your chosen protocol. Thanks to the integrated text interface, you can monitor connections and track traffic usage in real-time.

## Features

- **Multiple Protocol Support**:
  - SSH tunneling
  - SOCKS5 proxy
  - WireGuard VPN
- **Automatic DNS configuration**
- **Flexible routing** with IP address list support
- **Real-time monitoring**:
  - Active connections
  - Traffic usage statistics
  - Protocol information
- **Traffic type detection** to identify used protocols
- **IPv4 and IPv6** traffic handling

## Installation

Before installing, ensure `make` and Go are installed on your system. The application requires root privileges to create network interfaces.

```bash
# Clone the repository
git clone https://github.com/merzzzl/warp.git
cd warp

# Build for your platform
make build

# Alternatively, for macOS ARM64
make build-darwin-arm64
```

## Usage

Create a configuration file `~/.warp.yaml` in your home directory and launch WARP:

```bash
sudo ./warp
```

### Command Line Options

WARP supports the following command-line options:

- `--verbose`: Enable verbose logging for detailed operational logs (default: disabled)
- `--debug`: Enable debug logging with even more detailed information (default: disabled)
- `--fun`: Enable "magic" mode with colorful visualization (default: disabled)

### Configuration File

The `~/.warp.yaml` configuration file should contain tunnel and protocol settings:

```yaml
# Basic configuration
tunnel:
  name: utun11         # Name of the TUN interface to create
  ip: 192.168.127.0    # IP address to assign to the interface
  serve_dns: true      # ServeDNS allow to swap system dns to warp dns

# Connection protocols (only one protocol in each list item is used)
protocols:
  - ssh:               # SSH tunnel configuration
      # ...SSH parameters...
  - socks5:            # SOCKS5 proxy configuration
      # ...SOCKS5 parameters...
  - wireguard:         # WireGuard VPN configuration
      # ...WireGuard parameters...

# Optional
ipv6: true            # Allow IPv6 traffic handling (default: false)
```

## Configuration Examples

### SSH Tunnel

```yaml
tunnel:
  name: utun11
  ip: 192.168.127.0
protocols:
  - ssh:
      user: username            # SSH username
      password: password123     # Password (SSH keys recommended)
      host: example.com         # SSH server host
      domains:                  # Domains for DNS queries via tunnel
        - corp.example.com 
      dns:                      # Optional: DNS servers to use
        - 8.8.8.8
      ips:                      # Optional: Subnet list for routing
        - 10.0.0.0/8
        - 172.16.0.0/12
```

### SOCKS5 Proxy

```yaml
tunnel:
  name: utun11
  ip: 192.168.127.0
protocols:
  - socks5:
      user: username                # Optional: SOCKS5 username
      password: password123         # Optional: SOCKS5 password
      host: proxy.example.com:1080  # SOCKS5 proxy server address with port
      domains:                      # Domains for DNS queries via tunnel
        - corp.example.com 
      dns:                          # DNS servers
        - 8.8.8.8
      ips:                          # Subnet list for routing
        - 192.168.1.0/24
```

### WireGuard VPN

```yaml
tunnel:
  name: utun11
  ip: 192.168.127.0
protocols:
  - wireguard:
      private_key: aCZ7I1s+...        # WireGuard private key (Base64)
      peer_public_key: o78dBF...      # Peer public key (Base64)
      endpoint: wg.example.com:51820  # WireGuard server address with port
      domains:                        # Domains for DNS queries via tunnel
        - corp.example.com 
      address: 10.66.66.2             # IP address for WireGuard interface
      dns:                            # WireGuard DNS servers
        - 10.66.66.1
      ips:                            # Subnet list for routing
        - 10.66.66.0/24
```

## Monitoring

WARP includes a text-based user interface (TUI) for monitoring that shows:

- **Logs**: Current system messages and events
- **Connections**: Active network connections, their direction, and protocols
- **Bandwidth**: Current and cumulative data transfer statistics
- **IP List**: Routed IP addresses
- **Uptime**: Application runtime duration

## License

WARP is licensed under the [MIT License](LICENSE), supporting open and collaborative development.