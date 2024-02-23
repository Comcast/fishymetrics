# fishymetrics Change Log

All notable changes to this project will be documented in this file. This
project adheres to [Semantic Versioning](http://semver.org/) and this change
log is based on the [Keep a CHANGELOG](http://keepachangelog.com/) project.

## Unreleased
- Added HPE DL380 Gen10 support
- Enhanced drive metrics collection for DL380 model servers to include NVME, Storage Disk Drives, and Logical Drives
- Added HPE DL560 Gen9 support

## Fixed
- Cisco UCS C220 - add additional edge cases when collecting memory metrics [#2](https://github.com/Comcast/fishymetrics/issues/2)

## Updated
- removed references to internal URLs/FQDNs to opensource the project

## [0.7.1]

## Added
- added a mux prometheus middleware to collect and export metrics for every http request

## Fixed
- fix route issue from the /ignored html template

## [0.7.0]

## Fixed
- fixed Horizontal Pod Autoscaling k8s resource in helm chart

## Updated
- move buildinfo package to inside the fishymetrics repo
- update all go dependencies in project to remove any potential security bugs

## [0.6.16]

## Added
- add Horizontal Pod Autoscaling capabilities
- add ability to customize container resource limits/requests

## Fixed
- route prefix for metrics and info API paths

## Removed
- remove route prefix configuration

## Updated
- rename app container port name to exporter from metrics
- improve README documentation
- Add build info to the root home page
- use golang version 1.19.x

## [0.6.15]

## Changed
- Modified vector config in the helm chart to fix structured json log messages to elastic

## [0.6.14]

## Added
- added trace_id to all logging messages

## Changed
- fixed for loop logic for a targets scrape
- updated vector config to include a json remap transform

## [0.6.13]

## Added
- added ability to forward logs to an elastic cluster using vector

## Changed
- changed logging from oyez to zap package

## [0.6.12]

## Added
- add BIOS version to device_info metric
- add more labels to cisco device metrics to help with RMA automation

## Changed
- incease scrape timeout to 90 seconds for c220 devices
- update helm chart to reflect updated env vars

## Fixed
- fix CI bug with Dockerfile
- add DISABLED state for power and drive metric scrapes
- add DISABLED state for memory and processor metric scrapes

## [0.6.3]

## Added
- Added metrics for C220 storage/raid controllers and drives when applicable

## [0.6.2]

## Changed
- Change *url* label to be *name* and use the url path base for *name* label value

## [0.6.1]

## Added
- Added storage controller status metric for all cisco modules
- Added overall temperature status metric for all cisco modules

## Fixed
- Fix s3260m4 exporter module scrape endpoints
- Fix retry logic for certain cisco redfish API calls

## [0.6.0]

## Added
- Add vault integration for chassis credentials
- Add graceful shutdown of newly added go routines

## [0.5.1]

## Changed
- Temporarily removed drive scrapes from Cisco devices until we figure out the best plan forward

## [0.5.0]

## Added
- Create new prometheus exporters for Cisco UCS C220, S3260 M4, and S3260 M5 devices

## [0.4.1]

## Added
- Added support for DL20 devices

## Fixed
- Fix nil pointer reference for when module name in scrape request does not exist

## [0.4.0]

## Added
- Add support for scrapes to HP DL360s w/ iLO 5

## [0.3.1]

## Fixed
- Metrics are not reseting the way it used to
- Web UI not routing correctly when app is behind nginx-ingress

## [0.3.0]

## Changed
- Centralize fishymetrics exporter to handle more than 1 scrape endpoints

## [0.2.0]

## Added
- Add moonshot switch metrics collection for status, thermal, and power

## [0.1.1]

## Added
- Created Helm chart for deployment
- Add limiter and route-prefix flags/env variables

## [0.1.0]

## Added
- Initial commit of fishymetrics exporter
