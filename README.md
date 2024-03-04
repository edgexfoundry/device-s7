# Device Service for Siemens S7 PLC

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
