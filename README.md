# Device Service for Siemens S7 PLC

<!---
[![Build Status](https://jenkins.edgexfoundry.org/view/EdgeX%20Foundry%20Project/job/edgexfoundry/job/device-s7/job/main/badge/icon)](https://jenkins.edgexfoundry.org/view/EdgeX%20Foundry%20Project/job/edgexfoundry/job/device-s7/job/main/) [![Code Coverage](https://codecov.io/gh/edgexfoundry/device-s7/branch/main/graph/badge.svg?token=IUywg34zfH)](https://codecov.io/gh/edgexfoundry/device-s7) [![Go Report Card](https://goreportcard.com/badge/github.com/edgexfoundry/device-s7)](https://goreportcard.com/report/github.com/edgexfoundry/device-s7) [![GitHub Latest Dev Tag)](https://img.shields.io/github/v/tag/edgexfoundry/device-s7?include_prereleases&sort=semver&label=latest-dev)](https://github.com/edgexfoundry/device-s7/tags) ![GitHub Latest Stable Tag)](https://img.shields.io/github/v/tag/edgexfoundry/device-s7?sort=semver&label=latest-stable) [![GitHub License](https://img.shields.io/github/license/edgexfoundry/device-s7)](https://choosealicense.com/licenses/apache-2.0/) ![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/edgexfoundry/device-s7) [![GitHub Pull Requests](https://img.shields.io/github/issues-pr-raw/edgexfoundry/device-s7)](https://github.com/edgexfoundry/device-s7/pulls) [![GitHub Contributors](https://img.shields.io/github/contributors/edgexfoundry/device-s7)](https://github.com/edgexfoundry/device-s7/contributors) [![GitHub Committers](https://img.shields.io/badge/team-committers-green)](https://github.com/orgs/edgexfoundry/teams/device-s7-committers/members) [![GitHub Commit Activity](https://img.shields.io/github/commit-activity/m/edgexfoundry/device-s7)](https://github.com/edgexfoundry/device-s7/commits))
-->

> **Warning**  
> The **main** branch of this repository contains work-in-progress development code for the upcoming release, and is **not guaranteed to be stable or working**.
> It is only compatible with the [main branch of edgex-compose](https://github.com/edgexfoundry/edgex-compose) which uses the Docker images built from the **main** branch of this repo and other repos.
>
> **The source for the latest release can be found at [Releases](https://github.com/edgexfoundry/device-s7/releases).**

## Documentation

For latest documentation please visit https://docs.edgexfoundry.org/latest/microservices/device/services/device-s7/Purpose

## Overview

S7 Micro Service - device service for connecting Siemens S7(S7-200, S7-300, S7-400, S7-1200, S7-1500) devices by `ISO-on-TCP` to EdgeX.

- This device service is contributed by [YIQISOFT](https://yiqisoft.cn)
- The sevice has tested on S7-300, S7-400, S7-1200

## Features

- Single Read and Write
- Multiple Read and Write
- High performance, more 2000 items per second(depends on S7 model)
  - Use `S7-Device01` sample device configuration, `interval` should be less than `IdelTimeout`
  - S7-1200 and S7-1500 preferred
  - Create multiple connections to one S7 device use different device name

## Prerequisites

- A Siemens S7 series device with network interface
- Enable ISO-on-TCP connection on the S7 device

## Build Instructions

1.  Clone the device-rest-go repo with the following command:

        git clone https://github.com/edgexfoundry/device-s7.git

2.  Build a docker image by using the following command:

        make docker

3.  Alternatively the device service can be built natively:

        make build

## Build with NATS Messaging

Currently, the NATS Messaging capability (NATS MessageBus) is opt-in at build time.
This means that the published Docker images do not include the NATS messaging capability.

The following make commands will build the local binary or local Docker image with NATS messaging capability included.

```shell
make build-nats
make docker-nats
```

## Packaging

This component is packaged as docker images.

Please refer to the [Dockerfile](./Dockerfile) and [Docker Compose Builder](https://github.com/edgexfoundry/edgex-compose/tree/main/compose-builder) scripts.

## Reference

- [Gos7](https://github.com/robinson/gos7)

## License

Apache-2.0
