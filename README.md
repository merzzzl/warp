# WARP

WARP is a tool designed to forward TCP traffic through an SSH tunnel. It provides a secure and efficient way to route your network data.

## Table of Contents

- [WARP](#warp)
  - [Table of Contents](#table-of-contents)
  - [Installation](#installation)
  - [Usage](#usage)
    - [Command Line Options](#command-line-options)
    - [Configuration File Options](#configuration-file-options)
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

To run WARP, create the `~/.warp.conf` file in the user home directory and run warp using the following command line options:

```bash
sudo ./warp [options]
```

### Command Line Options

- `-verbose`: Run WARP in console verbose logging mode. (dissable TUI mode)

### Configuration File Options

```yaml
# WARP configuration file example
---
tunnel:
  name: utun0
  ip: 192.168.200.0
ssh:
 user: admin
 password: p@ssw0rd
 host: 192.168.1.100
 domain: (.*\.)?example\.com$
```

## Examples

Here's an updated example demonstrating how to forward TCP traffic through an SSH tunnel, including routing to a Kubernetes network:

```bash
sudo ./warp
```

![WARP run with TUI mode](README.png)

## License

This project is licensed under the MIT License.
