# Device Service for Siemens S7 PLC

<!---
[![Build Status](https://jenkins.edgexfoundry.org/view/EdgeX%20Foundry%20Project/job/edgexfoundry/job/device-s7/job/main/badge/icon)](https://jenkins.edgexfoundry.org/view/EdgeX%20Foundry%20Project/job/edgexfoundry/job/device-s7/job/main/) [![Code Coverage](https://codecov.io/gh/edgexfoundry/device-s7/branch/main/graph/badge.svg?token=IUywg34zfH)](https://codecov.io/gh/edgexfoundry/device-s7) [![Go Report Card](https://goreportcard.com/badge/github.com/edgexfoundry/device-s7)](https://goreportcard.com/report/github.com/edgexfoundry/device-s7) [![GitHub Latest Dev Tag)](https://img.shields.io/github/v/tag/edgexfoundry/device-s7?include_prereleases&sort=semver&label=latest-dev)](https://github.com/edgexfoundry/device-s7/tags) ![GitHub Latest Stable Tag)](https://img.shields.io/github/v/tag/edgexfoundry/device-s7?sort=semver&label=latest-stable) [![GitHub License](https://img.shields.io/github/license/edgexfoundry/device-s7)](https://choosealicense.com/licenses/apache-2.0/) ![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/edgexfoundry/device-s7) [![GitHub Pull Requests](https://img.shields.io/github/issues-pr-raw/edgexfoundry/device-s7)](https://github.com/edgexfoundry/device-s7/pulls) [![GitHub Contributors](https://img.shields.io/github/contributors/edgexfoundry/device-s7)](https://github.com/edgexfoundry/device-s7/contributors) [![GitHub Committers](https://img.shields.io/badge/team-committers-green)](https://github.com/orgs/edgexfoundry/teams/device-s7-committers/members) [![GitHub Commit Activity](https://img.shields.io/github/commit-activity/m/edgexfoundry/device-s7)](https://github.com/edgexfoundry/device-s7/commits))
-->

## Overview

S7 Micro Service - device service for connecting Siemens S7(S7-200, S7-300, S7-400, S7-1200, S7-1500) devices by `ISO-on-TCP` to EdgeX.

## Features

- Single Read and Write
- Multiple Read and Write
- High performance, more 2000 items per second(depends on S7 model)
  - Use `S7-Device01` sample device configuration, `interval` should less than `IdelTimeout`
  - S7-1200 and S7-1500 preferred
  - Create multiple connections to one S7 device use different device name

## Prerequisites

- A Siemens S7 series device with network interface
- Enable ISO-on-TCP connection

### Install and Deploy

Clone and Build

```shell
git clone https://github.com/edgexfoundy-holding/device-s7.git
cd device-s7
make build
```

To start the device service:

```shell
export EDGEX_SECURITY_SECRET_STORE=false
make run
```

To rebuild after making changes to source:

```shell
make clean
make build
```

## Packaging

This component is packaged as Docker image.

For docker, please refer to the `Dockerfile`.

### Build Docker image

```shell
make docker
```

The docker image looks like:

```
edgexfoundry/device-s7    3.1.0-dev    c84a77f45860   5 minutes ago   33.6MB
```

### Docker compose file

Add to your docker-compose.yml.

```yaml
device-s7:
  container_name: edgex-device-s7
  depends_on:
    consul:
      condition: service_started
      required: true
    core-data:
      condition: service_started
      required: true
    core-metadata:
      condition: service_started
      required: true
  environment:
    EDGEX_SECURITY_SECRET_STORE: 'false'
    SERVICE_HOST: edgex-device-s7
  hostname: edgex-device-s7
  image: edgexfoundry/device-s7:0.0.0-dev
  networks:
    edgex-network: null
  ports:
    - mode: ingress
      host_ip: 127.0.0.1
      target: 59994
      published: '59994'
      protocol: tcp
  read_only: true
  restart: always
  security_opt:
    - no-new-privileges:true
  user: 2002:2001
  volumes:
    - type: bind
      source: /etc/localtime
      target: /etc/localtime
      read_only: true
      bind:
        create_host_path: true
```

## Usage

### Device Profile Sample

You should change all `valueType`, `NodeName` and `name` to your real `devices/device.yml`.

- `NodeName` should be use `DB4.DBX0.0` format

```yaml
nname: S7-Device
manufacturer: YIQISOFT
description: Example of S7 Device
model: Siemens S7
labels: [ISO-on-TCP]
deviceResources:
  - name: bool
    description: PLC bool
    isHidden: false
    properties:
      valueType: Bool
      readWrite: RW
    attributes:
      NodeName: DB4.DBX0.0
  - name: byte
    description: PLC byte
    isHidden: false
    properties:
      valueType: Uint8
      readWrite: RW
    attributes:
      NodeName: DB4.DBB1
  - name: word
    description: PLC word
    isHidden: false
    properties:
      valueType: Int16
      readWrite: RW
    attributes:
      NodeName: DB4.DBW2
  - name: dword
    description: PLC dword
    isHidden: false
    properties:
      valueType: Int32
      readWrite: RW
    attributes:
      NodeName: DB4.DBD4
  - name: int
    description: PLC int
    isHidden: false
    properties:
      valueType: Int16
      readWrite: RW
    attributes:
      NodeName: DB4.DBW8
  - name: dint
    description: PLC dint
    isHidden: false
    properties:
      valueType: Int32
      readWrite: RW
    attributes:
      NodeName: DB4.DBW10
  - name: real
    description: PLC real
    isHidden: false
    properties:
      valueType: Float32
      readWrite: RW
    attributes:
      NodeName: DB4.DBD14
  - name: heartbeat
    description: PLC heartbeat
    isHidden: false
    properties:
      valueType: Int16
      readWrite: RW
    attributes:
      NodeName: DB1.DBW160
deviceCommands:
  - name: AllResource
    isHidden: false
    readWrite: RW
    resourceOperations:
      - deviceResource: bool
        defaultValue: 'false'
      - deviceResource: byte
        defaultValue: '0'
      - deviceResource: word
        defaultValue: '0'
      - deviceResource: dword
        defaultValue: '0'
      - deviceResource: int
        defaultValue: '0'
      - deviceResource: dint
        defaultValue: '0'
      - deviceResource: real
        defaultValue: '0'
      - deviceResource: heartbeat
        defaultValue: '0'
```

### Device Sample

Change `Host`, `Port`, `Rack`, `Slot`, `Timeout`, `IdleTimeout`, `interval` and others(if you want) to your real `Configuration`.

#### S7 protocol in EdgeX device:

| Key         | Default value | Note                             |
| ----------- | ------------- | -------------------------------- |
| Host        | N/A           | S7 ip address                    |
| Port        | N/A           | S7 port, e.g. 102                |
| Rack        | 0             | Rack number                      |
| Slot        | 1             | Slot number                      |
| Timeout     | 30            | connect to S7 timeout, seconds   |
| IdleTimeout | 30            | connection idle timeout, seconds |

#### Sample devices for test

- **S7-Device01 for test `interval` less than `IdleTimeout`**
  All `AutoEvent`s in one tcp connection.

- **S7-Device02 for test `interval` grater than `IdleTimeout`**
  Release tcp connection and disconnect while idle timeout, it will reconnect in the next interval period.

```yaml
deviceList:
  - name: S7-Device01
    profileName: S7-Device
    description: Example of S7 Device
    labels: [industrial]
    protocols:
      s7:
        Host: 192.168.123.199
        Port: 102
        Rack: 0
        Slot: 1
        Timeout: 30
        IdleTimeout: 30
    autoEvents:
      - interval: 10s
        onChange: false
        sourceName: AllResource
  - name: S7-Device02
    profileName: S7-Device
    description: Example of S7 Device
    labels: [industrial]
    protocols:
      s7:
        Host: 192.168.123.199
        Port: 102
        Rack: 0
        Slot: 1
        Timeout: 5
        IdleTimeout: 5
    autoEvents:
      - interval: 10s
        onChange: false
        sourceName: heartbeat
```

### Service status

#### Sevice Ping

```shell
curl http://localhost:59994/api/v3/ping
```

```json
{
  "apiVersion": "v3",
  "timestamp": "Wed Oct 18 10:45:49 UTC 2023",
  "serviceName": "device-s7"
}
```

#### Get version

```shell
curl http://localhost:59994/api/v3/version
```

```json
{
  "apiVersion": "v3",
  "version": "3.1.0",
  "serviceName": "device-s7",
  "sdk_version": "0.0.0"
}
```

### Execute Commands

#### All device

```shell
curl http://localhost:59882/api/v3/device/all
```

```json
{
  "apiVersion": "v3",
  "deviceCoreCommands": [
    {
      "coreCommands": [
        {
          "get": true,
          "name": "AllResource",
          "parameters": [
            {
              "resourceName": "bool",
              "valueType": "Bool"
            },
            {
              "resourceName": "byte",
              "valueType": "Uint8"
            },
            {
              "resourceName": "word",
              "valueType": "Int16"
            },
            {
              "resourceName": "dword",
              "valueType": "Int32"
            },
            {
              "resourceName": "int",
              "valueType": "Int16"
            },
            {
              "resourceName": "dint",
              "valueType": "Int32"
            },
            {
              "resourceName": "real",
              "valueType": "Float32"
            },
            {
              "resourceName": "heartbeat",
              "valueType": "Int16"
            }
          ],
          "path": "/api/v3/device/name/S7-Device01/AllResource",
          "set": true,
          "url": "http://edgex-core-command:59882"
        },
        {
          "get": true,
          "name": "byte",
          "parameters": [
            {
              "resourceName": "byte",
              "valueType": "Uint8"
            }
          ],
          "path": "/api/v3/device/name/S7-Device01/byte",
          "set": true,
          "url": "http://edgex-core-command:59882"
        },
        {
          "get": true,
          "name": "word",
          "parameters": [
            {
              "resourceName": "word",
              "valueType": "Int16"
            }
          ],
          "path": "/api/v3/device/name/S7-Device01/word",
          "set": true,
          "url": "http://edgex-core-command:59882"
        },
        {
          "get": true,
          "name": "dword",
          "parameters": [
            {
              "resourceName": "dword",
              "valueType": "Int32"
            }
          ],
          "path": "/api/v3/device/name/S7-Device01/dword",
          "set": true,
          "url": "http://edgex-core-command:59882"
        },
        {
          "get": true,
          "name": "int",
          "parameters": [
            {
              "resourceName": "int",
              "valueType": "Int16"
            }
          ],
          "path": "/api/v3/device/name/S7-Device01/int",
          "set": true,
          "url": "http://edgex-core-command:59882"
        },
        {
          "get": true,
          "name": "dint",
          "parameters": [
            {
              "resourceName": "dint",
              "valueType": "Int32"
            }
          ],
          "path": "/api/v3/device/name/S7-Device01/dint",
          "set": true,
          "url": "http://edgex-core-command:59882"
        },
        {
          "get": true,
          "name": "heartbeat",
          "parameters": [
            {
              "resourceName": "heartbeat",
              "valueType": "Int16"
            }
          ],
          "path": "/api/v3/device/name/S7-Device01/heartbeat",
          "set": true,
          "url": "http://edgex-core-command:59882"
        },
        {
          "get": true,
          "name": "bool",
          "parameters": [
            {
              "resourceName": "bool",
              "valueType": "Bool"
            }
          ],
          "path": "/api/v3/device/name/S7-Device01/bool",
          "set": true,
          "url": "http://edgex-core-command:59882"
        },
        {
          "get": true,
          "name": "real",
          "parameters": [
            {
              "resourceName": "real",
              "valueType": "Float32"
            }
          ],
          "path": "/api/v3/device/name/S7-Device01/real",
          "set": true,
          "url": "http://edgex-core-command:59882"
        }
      ],
      "deviceName": "S7-Device01",
      "profileName": "S7-Device"
    }
  ],
  "statusCode": 200,
  "totalCount": 1
}
```

#### Set command

```shell
curl http://localhost:59882/api/v3/device/name/S7-Device01/heartbeat \
-X PUT \
-H "Content-Type:application/json" \
-d '{"heartbeat": "1"}'
```

```json
{
  "apiVersion": "v3",
  "statusCode": 200
}
```

#### Get command

```shell
curl http://localhost:59882/api/v3/device/name/S7-Device01/heartbeat
```

```json
{
  "apiVersion": "v3",
  "statusCode": 200,
  "event": {
    "apiVersion": "v3",
    "id": "7c85a003-7d82-4507-815f-85895c3c758f",
    "deviceName": "S7-Device01",
    "profileName": "S7-Device",
    "sourceName": "heartbeat",
    "origin": 1697626327949066679,
    "readings": [
      {
        "id": "9bb71412-1416-47df-be38-f4bda4aa258b",
        "origin": 1697626327948986054,
        "deviceName": "S7-Device01",
        "resourceName": "heartbeat",
        "profileName": "S7-Device",
        "valueType": "Int16",
        "value": "1"
      }
    ]
  }
}
```

## Reference

- [Gos7](https://github.com/robinson/gos7)

## License

Apache-2.0
