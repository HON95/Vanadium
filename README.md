# Vanadium

[![GitHub release](https://img.shields.io/github/v/release/HON95/vanadium?label=Version)](https://github.com/HON95/vanadium/releases)
[![CI](https://github.com/HON95/vanadium/workflows/CI/badge.svg?branch=master)](https://github.com/HON95/vanadium/actions?query=workflow%3ACI)
[![FOSSA status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FHON95%2Fvanadium.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FHON95%2Fvanadium?ref=badge_shield)
[![Docker pulls](https://img.shields.io/docker/pulls/hon95/vanadium?label=Docker%20Hub)](https://hub.docker.com/r/hon95/vanadium)

**TODO** Description.

## Usage

### Docker

Use `1` for stable v1.Y.Z releases and `latest` for bleeding/unstable releases.

Example `docker-compose.yml`:

```yaml
services:
  ethermine-exporter:
    image: hon95/vanadium:1
    #command:
    #  - '--endpoint=:8080'
    #  - '--debug'
    user: 1000:1000
    environment:
      - TZ=Europe/Oslo
    ports:
      - "8080:8080/tcp"
```

## Configuration

**TODO**

## Scrapers

| Scraper | Supported Devices | L2 | L3 | Comments |
| - | - | - | - | - |
| Linux | Debian | No | Yes | **TODO** |
| VyOS SSH | VyOS | No | Yes | |
| Junos EX SSH | Juniper EX switches | Yes | No | |
| FSOS SSH | FS S5860-20SQ | Yes | Yes | **TODO** |

## Metrics

**TODO**

## Miscellanea

- MAC and IP addresses are normalized according to Go's [ParseMAC](https://pkg.go.dev/net#ParseMAC) and [ParseIP](https://pkg.go.dev/net#ParseIP).

## Development

- Build: `go build -o vanadium cmd/vanadium/*.go`
- Build and run (Docker): `docker-compose up --build`
- Vet: `go vet ./...`
- Lint: `golint ./...`

## TODO

- Distribute scrapes during scrape interval.
- Scrape timeout.
- VRFs/VPNs. Just show data for all VRFs now and ignore VRF names. If a VRF is not currently included in the data, it's considered a bug.
- MAC table: Junos stores for up to 5 minutes.
- Neighbors: Linux defaults to 30s from reachable to stale in neighbor table. Scraping interval should thus be <30s to capture everything. Junos?
- Per-device scraping interval?
- Per-device scraping interval and goroutine, allows easy offset intervals. Remove "scrapeAll".
- Tune so that number of devices actually matches real users.
- Aggregation: Remove overlapping MAC/IP addresses. Remove scraped device addresse (we got both MAC and IP?).
- Use NETCONF XML for Junos instead of CLI screen scraping.
- Prom metrics:
  - Device info. Always 1.
  - Last scrape status per device, 1 for successful.
  - Last scrape duration per device.
  - Number of devices. Per type.
  - Note: How to separate user devices from non-user devices. Can't use source devices only, not all devices are scraped. Provice ignore list of IP and MAC addresses?
  - MAC addresses per VLAN ID. Remove duplicates. Include sources?
  - IP addresses per subnet. Remove duplicates.

## License

GNU General Public License version 3 (GPLv3).
