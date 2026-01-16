# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-01-16

### Added
- **Parallel execution**: New `repeat` and `batch` commands for running scenarios multiple times with configurable parallelism (`-n`, `-p` flags)
- **Batch execution**: New `netroub batch` command for running multiple scenarios from a plan file (YAML/JSON) with glob pattern support
- **Clean subcommand**: New `netroub clean` command for removing containers left by interrupted executions, with `--dry-run` option
- **Progress display**: New `--progress` flag showing real-time progress bar with ETA, suppressing detailed logs
- **Batch log file**: Execution log (`netroub.log`) recording all task completions and summary for batch/repeat commands
- **Mutex queueing**: Deploy/destroy operations serialized to avoid netlink race conditions while events execute in parallel
- **Dynamic subnet allocation**: Subnet size calculated based on device count (supports networks from /30 to /16)
- **Unified file format**: Both scenario and plan files support JSON and YAML with automatic detection
- **Content-based file type detection**: `clean` command auto-detects Plan vs Scenario based on content
- **Scenario format auto-detection**: `--yaml` flag now optional; format detected by file extension
- **noDeploy mode**: Scenarios without `topo` field skip deploy/destroy, running events only on existing containers
- **Collect event**: New event type for collecting files from containers to trial log directory
- **Copy event**: New event type for copying files to/from containers (`copy_to`, `copy_from`)
- **Shell event**: New event type for executing shell commands in containers
- **Failure tracking**: Failed tasks show log directory path in summary for easier debugging
- **GitHub Actions**: Automated release workflow for Linux amd64/arm64 binaries

### Fixed
- **Tcpdump log directory**: Use `MkdirAll` to create parent directories when saving tcpdump logs

### Improved
- **Error messages**: Enhanced with detailed context (container name, file paths, command output)
- **Log output timing**: Deploy/destroy logs now appear after mutex acquisition, reflecting actual execution order

### Changed
- **CLI structure**: Refactored to subcommand structure (`run`, `repeat`, `batch`, `clean`) while maintaining backward compatibility

## [0.0.2] - 2025-08-02

### Added
- **Stress command**: Pumba stress command for CPU/memory stress testing (PR #6)
- **Rate command**: Pumba rate command for bandwidth limiting (PR #5)
- **Data path in scenario**: Added `data` field to scenario file for specifying dot2net data.json path (PR #3)

### Fixed
- **DestroyNetwork function**: Fixed error preventing network from being destroyed properly (PR #4)
- **TcpdumpLog function**: Ensure log directory is created before writing log files (PR #2)

### Changed
- **Scenario file structure**: Updated scenario file format with clear paths for topo, data, and logPath
- **Example files**: Updated example scenario files with correct paths

## [0.0.1] - 2023-09-22

### Added
- Initial release
- Basic scenario execution with JSON/YAML support
- Containerlab integration for network emulation
- Pumba integration for network fault injection (delay, loss, corrupt, duplicate)
- tcpdump logging on target hosts
- Log file collection after scenario completion

[Unreleased]: https://github.com/cpflat/netroub/compare/v0.0.2...HEAD
[0.0.2]: https://github.com/cpflat/netroub/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/cpflat/netroub/releases/tag/v0.0.1
