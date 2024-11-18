# WARP

WARP offering an advanced solution for forwarding TCP traffic through an SSH tunnel with the added advantage of a free VPN over WireGuard. This integration aims to enhance the security and efficiency of your network data routing.

## Table of Contents

- [Introduction](#introduction)
- [Installation](#installation)
- [Usage](#usage)
  - [Command Line Options](#command-line-options)
  - [Configuration File Options](#configuration-file-options)
- [Examples](#examples)
- [License](#license)

## Introduction

With the latest update, WARP consolidates its functionality by not only forwarding TCP traffic through an SSH tunnel but also by offering seamless integration with WireGuard. This makes WARP a comprehensive network security tool.

## Installation

Before installing, make sure `make` is available on your system. Follow these steps to install:

```bash
git clone https://github.com/merzzzl/warp.git
cd warp
make build

```

## Usage

To use WARP, begin by creating a ~/.warp.yaml file in your home directory. Launch WARP with the following command, adjusting the options as needed::

```bash
sudo ./warp --verbose
```

### Command Line Options

Currently, WARP supports the following command-line option:

- `-verbose`: Enable verbose logging to get detailed operational logs (default: disabled).

### Configuration File Options

Below is a template for the WARP configuration file, reflecting the current structure. Replace the placeholders with your actual data:

```yaml
# WARP configuration file example
---
tunnel:
  name: utun11
  ip: 192.168.127.0
protocols:
- ssh:
    user: <USER>
    password: <PASSWORD>
    host: <HOST>
    domain: .*example\.com$
    ips:
    - 10.0.0.0/8
```

## Examples

Here's an updated example demonstrating how to forward TCP traffic through an SSH tunnel:

```bash
sudo ./warp
```

![WARP run with TUI mode](README.png)

## License

WARP is licensed under the MIT License, supporting open and collaborative development.
