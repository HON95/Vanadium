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
- Lint: `golint ./...`

## TODO

- Clean shutdown of web server, scraper and DB?
- Distribute scrapes during scrape interval.
- Group L2/MAC by VLAN ID, group L3/IP by network.
- VRFs. Just show data for all VRFs now and ignore VRF names. If a VRF is not currently included in the data, it's considered a bug.

## License

GNU General Public License version 3 (GPLv3).
