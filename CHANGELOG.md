# fishymetrics Change Log

All notable changes to this project will be documented in this file. This
project adheres to [Semantic Versioning](http://semver.org/) and this change
log is based on the [Keep a CHANGELOG](http://keepachangelog.com/) project.

## Unreleased

## [0.13.1]

## Fixed

- include nil reference check during retires [#114](https://github.com/Comcast/fishymetrics/pull/114)
- Fix leaks in go routines and handling of response body [#115](https://github.com/Comcast/fishymetrics/pull/115)

## Updated
- Bump golang.org/x/net from 0.25.0 to 0.33.0 [#113](https://github.com/Comcast/fishymetrics/pull/113)
- Bump golang.org/x/crypto from 0.23.0 to 0.31.0 [#110](https://github.com/Comcast/fishymetrics/pull/110)

## [0.13.0]

## Updated

- increase security context for kubernetes helm chart deployment [#102](https://github.com/Comcast/fishymetrics/issues/102)
- metric names in moonshot exporter to generic names as seen in other models [#106](https://github.com/Comcast/fishymetrics/issues/106)

## [0.12.1]

## Fixed

- nil pointer dereference during a scrape [#97](https://github.com/Comcast/fishymetrics/issues/97)

## Updated

 - Removed trailing spaces in label values [#96](https://github.com/Comcast/fishymetrics/issues/96)
 - Add missing language fences to README [#95](https://github.com/Comcast/fishymetrics/pull/95)
 - Set Fishymetrics port default to 10023 [#100](https://github.com/Comcast/fishymetrics/issues/100)

## [0.12.0]

## Added

- Add ability to reference different vault paths for credential retrieval [#25](https://github.com/Comcast/fishymetrics/issues/25)
- Added HPE DL380 Gen10 support [#17](https://github.com/Comcast/fishymetrics/issues/17)
- Enhanced drive metrics collection for DL380 model servers to include NVME, Storage Disk Drives, and Logical Drives [#17](https://github.com/Comcast/fishymetrics/issues/17)
- Add ability to send logs directly to elasticsearch endpoints [#10](https://github.com/Comcast/fishymetrics/issues/10)
- Add HPE Proliant DL560 Gen9 support [#23](https://github.com/Comcast/fishymetrics/issues/23)
- Add HPE Proliant XL420 Support [#33](https://github.com/Comcast/fishymetrics/issues/33)
- consolidate exporters into a single generic exporter [#52](https://github.com/Comcast/fishymetrics/issues/52)
- update Dockerfile to comply with opensource packaging requirements [#61](https://github.com/Comcast/fishymetrics/issues/61)
- Storage controller status metric for HP servers [#79](https://github.com/Comcast/fishymetrics/issues/79)
- Ignore CPU metrics if Processor is Absent [#79](https://github.com/Comcast/fishymetrics/issues/79)
- Added support for metrics collection from Dell servers [#77](https://github.com/Comcast/fishymetrics/issues/77)
- Added support for firmware metrics collection from all supported servers and iLO versions from a single universal exporter [#83](https://github.com/Comcast/fishymetrics/issues/83)
- Added support for Supermicro models metrics collection [#87](https://github.com/Comcast/fishymetrics/issues/87)
- Added option to skip/unskip TLS verification [#94](https://github.com/Comcast/fishymetrics/issues/94)

## Fixed

- Cisco UCS C220 - add additional edge cases when collecting memory metrics [#2](https://github.com/Comcast/fishymetrics/issues/2)
- null pointer derefence errors when using incorrect credentials [#36](https://github.com/Comcast/fishymetrics/issues/36)
- incorrect /Memory path for HPE hosts [#49](https://github.com/Comcast/fishymetrics/issues/49)
- Thermal Summary, Power total consumed for Cisco servers and Chassis, memory metrics for Gen9 server models [#53](https://github.com/Comcast/fishymetrics/issues/53)
- Firmware gathering endpoint update and add device info to other HP models [#55](https://github.com/Comcast/fishymetrics/issues/55)
- C220 drive metrics on hosts with fw < 4.0, psu metrics result and label values [#57](https://github.com/Comcast/fishymetrics/issues/57)
- Chassis ComputerSystems field is handled improperly [#68](https://github.com/Comcast/fishymetrics/issues/68)
- Power and Thermal metrics collection for Dell R7xxXD server models [#77](https://github.com/Comcast/fishymetrics/issues/77)
- Firmware metrics and request headers update for Dell iDRAC9 with FW ver.3.xx and 4.xx [#77](https://github.com/Comcast/fishymetrics/issues/77)
- Power supply status duplicate bay number metrics [#85](https://github.com/Comcast/fishymetrics/issues/85)
- Capturing Model field in finished scrape log message [#94](https://github.com/Comcast/fishymetrics/issues/94)

## Updated

- Enhanced drive metrics collection for HPE DL360 model servers to include NVME, Storage Disk Drives, and Logical Drives. [#31](https://github.com/Comcast/fishymetrics/issues/31)
- Removed references to internal URLs/FQDNs to opensource the project
- Cisco S3260M5 module to support FW Ver 4.2(xx) [#18](https://github.com/Comcast/fishymetrics/issues/18)
- HP DL360 module to support responses from iLO4 [#34](https://github.com/Comcast/fishymetrics/issues/34)
- HP DL360 & XL420 to include processor, iloselftest and smart storage battery metrics [#43](https://github.com/Comcast/fishymetrics/issues/43)
- consolidate hardware component structs to a single package [#45](https://github.com/Comcast/fishymetrics/issues/45)
- get chassis serial number from JSON response instead of url path [#50](https://github.com/Comcast/fishymetrics/issues/50)
- HP DL380 module to include CPU metrics and all HP models to include bayNumber in PSU metrics [#57](https://github.com/Comcast/fishymetrics/issues/57)
- use standard library for http routing instead of gorilla mux package [#47](https://github.com/Comcast/fishymetrics/issues/47)
- Avoid collecting firmware metrics if count of endpoints are 75 or greater [#77](https://github.com/Comcast/fishymetrics/issues/77)
- Support for physical disk, logical drive and storage controller metrics collection from iLO5 fw ver.3.0.x [#91](https://github.com/Comcast/fishymetrics/issues/91)
- Updated vault api, sdk and containerd packages to fix known security issues [#94](https://github.com/Comcast/fishymetrics/issues/94)
- Deprecate module query param in favor of model [#94](https://github.com/Comcast/fishymetrics/issues/94)

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

- Change _url_ label to be _name_ and use the url path base for _name_ label value

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
