# dmo-wrapminer
DynMiner wrapping utility to ease setting configuration options for DynMiner or SRBMiner and manage restarting the miner as neccesary.
It will auto-restart your miner every *X* seconds based on config or will automatically restart it any time it crashes.

dmo-wrapminer is required in order to connect your miners to dmo-monitor.com, but dmo-monitor use is NOT required, 
this can simply be used as a wrapper to restart your miner and locally manage configuration options at this time.

## Usage
* Using one of the provided example .yaml config files as a template, add or edit your actual configuration
* Save your configuration as `mydmowrapconfig.yaml` and it will AUTOMATICALLY be used by dmo-wrapminer if 
  no other config file is passed on the command line when it is run
* Place the executable and config in the same directory as your DynMiner executable and dyn_miner .cl OR SRBMiner
* Execute dmo-wrapminer and pass the name of your config file as the only argument, eg: 

`./dmo-wrapminer my_awesome_config.yaml`
OR 
just run dmo-wrapminer with no arguments if you have saved your configuration file as `mydmowrapconfig.yaml`

NOTE: If you wish to run dmo-wrapminer with the ability to AUTOMATICALLY UPDATE ITSELF then simply use the dmo-launcher instead of running dmo-wrapminer directly, eg:
`./dmo-launcher my_awesome_config.yaml`

