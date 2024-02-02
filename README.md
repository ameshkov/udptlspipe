[![Go Report Card](https://goreportcard.com/badge/github.com/ameshkov/udptlspipe)](https://goreportcard.com/report/ameshkov/udptlspipe)
[![Latest release](https://img.shields.io/github/release/ameshkov/udptlspipe/all.svg)](https://github.com/ameshkov/udptlspipe/releases)

# udptlspipe

`udptlspipe` is a very simple TLS wrapper for UDP sessions with active probing
protection. The main purpose of it is to wrap WireGuard, OpenVPN or any other
similar UDP sessions. Inspired by [dtlspipe][dtlspipe].

This tool is very simple and created to solve a very specific issue and I intend
to keep it that way.

* [Features](#features)
* [Why would you need it?](#why)
* [How to install udptlspipe](#install)
* [How to use udptlspipe](#howtouse)
* [All command-line arguments](#allcmdarguments)

[dtlspipe]: https://github.com/SenseUnit/dtlspipe

<a id="features"></a>

## Features

* Cross-platform (Windows/macOS/Linux/Android/*BSD).
* Simple configuration, no complicated configuration files.
* Mimics Google Chrome's TLS ClientHello.
* Active probing protection in server mode.
* Suitable to wrap WireGuard, OpenVPN, and other UDP session.

<a id="why"></a>

## Why would you need it

There are several use-cases when this tool may be useful, the most popular ones
are:

* You're in a network where UDP-based protocol (like WireGuard) is forbidden.
* You're in a network where UDP works unreliably.

<a id="install"></a>

## How to install udptlspipe

* Using homebrew:
    ```shell
    brew install ameshkov/tap/udptlspipe
    ```
* From source:
    ```shell
    go install github.com/ameshkov/udptlspipe@latest
    ```
* You can get a binary from the [releases page][releases].

[releases]: https://github.com/ameshkov/udptlspipe/releases

<a id="howtouse"></a>

## How to use udptlspipe

### Generic case

Let's assume you have the following setup:

* You have a server with a public IP address `1.2.3.4` (**tunnel server**).
* You also have a UDP service running on `2.3.4.5:8123` (**udp server**).
* You want to access the UDP server securely from your **local machine** and
  wrap your UDP datagrams with a TLS layer.

Run the following command on your **tunnel server**.

```shell
udptlspipe --server -l 0.0.0.0:443 -d 2.3.4.5:8123 -p SecurePassword
```

Now run the following command on your **local machine**:

```shell
udptlspipe -l 127.0.0.1:8123 -d 1.2.3.4:443 -p SecurePassword
```

Now instead of using **udp server** address on your local machine use
`127.0.0.1:8123`.

In the end here's the pipe that you have:

**you** → UDP → **local udptlspipe** → TLS → **tunnel server** → UDP → **udp
server**.

### WireGuard

`udptlspipe` setup is completely the same as for the generic case, but you also
need to make some adjustments to the WireGuard client configuration:

* Use the address of the `udptlspipe` client as an endpoint in your WireGuard
  client configuration.
* Add `MTU = 1280` to the `[Peer]` section of both WireGuard client and server
  configuration files.
* Exclude the `udptlspipe` server IP from `AllowedIPs` in the WireGuard client
  configuration. This [calculator][wireguardcalculator] may help you.

[wireguardcalculator]: https://www.procustodibus.com/blog/2021/03/wireguard-allowedips-calculator/

<a id="allcmdarguments"></a>

## All command-line arguments

```shell
Usage:
  udptlspipe [OPTIONS]

Application Options:
  -s, --server                                              Enables the server mode. By default it runs in client mode.
  -l, --listen=<IP:Port>                                    Address the tool will be listening to (required).
  -d, --destination=<IP:Port>                               Address the tool will connect to (required).
  -p, --password=<password>                                 Password is used to detect if the client is allowed.
  -x, --proxy=[protocol://username:password@]host[:port]    URL of a proxy to use when connecting to the destination address
                                                            (optional).
  -v, --verbose                                             Verbose output (optional).

Help Options:
  -h, --help                                                Show this help message
```

## TODO

* [ ] Automatic TLS certs generation (let's encrypt, lego)
* [ ] Docker image