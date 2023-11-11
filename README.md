# WARP

WARP is a tool designed to forward TCP traffic through an SSH tunnel. It provides a secure and efficient way to route your network data, including support for routing traffic to Kubernetes networks.

## Table of Contents

- [WARP](#warp)
  - [Table of Contents](#table-of-contents)
  - [Installation](#installation)
  - [Usage](#usage)
    - [Command Line Options](#command-line-options)
  - [Examples](#examples)
  - [License](#license)

## Installation

To install WARP, you need to clone the repository and then build the project using `make`.

```bash
git clone https://github.com/merzzzl/warp.git
cd warp
make build
```

## Usage

To run WARP, execute the following command with the appropriate options:

```bash
./warp [options]
```

### Command Line Options

Here are the updated command line options:

- `-s`: Specifies the SSH host to connect to. Default is `root@127.0.0.1`.
- `-t`: Specifies the name of the utun device. Default is `utun5`.
- `-i`: Specifies the IP address for the utun device. Default is `192.168.48.1`.
- `-d`: Specifies the domain suffix for routing. Default is `.`.
- `-n`: Specifies the Kubernetes namespace. Default is `default`.
- `-k`: Path to Kubernetes config file.
- `-l`: IP for local network in 24 mask. Default is `127.0.40.0`.
- `-u`: Enables Text-based User Interface (TUI) mode. Disabled by default.

## Examples

Here's an updated example demonstrating how to forward TCP traffic through an SSH tunnel, including routing to a Kubernetes network:

```bash
./warp -s root@127.0.0.1 -t utun5 -i 192.168.48.1 -d . -n default -k path/to/kubeconfig -l 127.0.40.0 -u
```


![WARP run with TUI mode](README.png)

## License

This project is licensed under the MIT License.
