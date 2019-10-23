[![Go Report Card](https://goreportcard.com/badge/github.com/Luzifer/scs-extract)](https://goreportcard.com/report/github.com/Luzifer/scs-extract)
![](https://badges.fyi/github/license/Luzifer/scs-extract)
![](https://badges.fyi/github/downloads/Luzifer/scs-extract)
![](https://badges.fyi/github/latest-release/Luzifer/scs-extract)
![](https://knut.in/project-status/scs-extract)

# Luzifer / scs-extract

`scs-extract` is a Linux / MacOS CLI util to list / extract files from SCS archives used in Euro Truck Simulator 2 / American Truck Simulator.

## Usage

`scs-extract [options] <archive> [files to extract]`

```console
# scs-extract ~/.steam/steam/steamapps/common/Euro\ Truck\ Simulator\ 2/def.scs def/economy_data.sii
def/economy_data.sii

# scs-extract --help
Usage of scs-extract:
  -d, --dest string        Path prefix to use to extract files to (default ".")
  -x, --extract            Extract files (if not given files are just listed)
      --log-level string   Log level (debug, info, warn, error, fatal) (default "info")
      --version            Prints current version and exits
```
