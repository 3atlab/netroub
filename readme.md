# Netroub

Netroub is a platform capable of emulating complex and realistic network trouble scenarios to collect synthetic operational data.
Netroub reproduces network failure behaviors described in a scenario file on Docker-based emulation networks.

## Overview

![form](image/netroub_overview.png)

## Requirements

- Docker
- [Pumba](https://github.com/alexei-led/pumba)
- [containerlab](https://containerlab.dev/)
- [dot2net](https://github.com/cpflat/dot2net) (optional)

## Installation

Download the binary from [GitHub Releases](https://github.com/3atlab/netroub/releases) (Linux amd64/arm64).

Or build from source:

    go build .

## Usage

### Run a single scenario

    netroub run scenario.json
    netroub run scenario.yaml

### Repeat execution

    netroub repeat scenario.json -n 100 -p 4

### Batch execution

    netroub batch plan.yaml -p 4 --progress

### Clean up containers

    netroub clean scenario.json    # single scenario
    netroub clean plan.yaml        # plan file

## Scenario file

| Field        | Type   | Description
|:-------------|:-------|----------------
| scenarioName | string | Name of the scenario
| logPath      | string | Path to the output directory
| topo         | string | Path to the topology file (containerlab)
| data         | string | Path to the data file with device parameters
| events       | array  | Array of events to execute

## Events

- **delay**, **loss**, **corrupt**, **duplicate**: Network fault injection (via Pumba)
- **rate**, **stress**: Bandwidth limiting and resource stress
- **shell**: Execute commands in containers
- **copy**: Copy files to/from containers
- **collect**: Collect files from containers to log directory

## Citation

If you use this tool in your research, consider citing our [CoNEXT 2023 student workshop publication](https://doi.org/10.1145/3630202.3630222).

``` bib
@inproceedings{colin_netroub,
  title={netroub: Towards an Emulation Platform for Network Trouble Scenarios},
  author={Colin Regal-Mezin, Satoru Kobayashi, Toshihiro Yamauchi},
  booktitle={Proceedings of the CoNEXT Student Workshop 2023 (CoNEXT-SW '23)},
  publisher={ACM},
  pages={17-18},
  year={2023},
}
```
