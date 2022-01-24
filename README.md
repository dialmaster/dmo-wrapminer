# dmo-wrapminer
DynMiner wrapping utility to ease setting configuration options for DynMiner and manage restarting the miner as neccesary.
It will auto-restart your miner every *X* seconds based on config or will automatically restart it any time it crashes.

This is an *alpha* release at this point.

Eventually it will be used in conjunction with https://github.com/dialmaster/dmo-monitor-binaries for full remote management of miner configuration,
restarts and deployments.

dmo-monitor use is NOT required, this can simply be used as a wrapper to restart your miner and locally manage configuration options at this time.

## Usage
* Copy `dmowrapconfig.yaml` to some non-default config filename, eg: `mydmowrapconfig.yaml` and edit/set configuration options
* Place the executable in the same directory as DynMiner2.exe and dyn_miner2.cl
* Execute dmo-wrapminer and pass the name of your config file as the only argument, eg: 

`./dmo-wrapminer.exe mydmowrapconfig.yaml`

## Building the executable
* Go Version used for build: 1.17.5

```
go get gopkg.in/yaml.v2
go build
```

