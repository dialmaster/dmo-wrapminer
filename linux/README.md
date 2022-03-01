# dmo-wrapminer
DynMiner wrapping utility to ease setting configuration options for DynMiner or SRBMiner and manage restarting the miner as neccesary.
It will auto-restart your miner every *X* seconds based on config or will automatically restart it any time it crashes.

dmo-wrapminer is required in order to connect your miners to dmo-monitor.com, but dmo-monitor use is NOT required, 
this can simply be used as a wrapper to restart your miner and locally manage configuration options at this time.

## Usage
* Copy `dmowrapconfig.yaml` to some non-default config filename, eg: `mydmowrapconfig.yaml` and edit/set configuration options
* Place the executable in the same directory as your DynMiner executable and dyn_miner .cl OR SRBMiner
* Execute dmo-wrapminer and pass the name of your config file as the only argument, eg: 

`./dmo-wrapminer.exe mydmowrapconfig.yaml`

