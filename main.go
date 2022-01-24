package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"time"
)

var configFile string
var m sync.Mutex
var mineCmd *exec.Cmd
var ttl time.Duration
var endMiner, endChkBlk time.Time
var user, pass, gpu, globalUnits, localUnits, GPUID, walletaddr string
var u *url.URL

/* DynMiner2.exe args for reference:
-mode [solo|stratum]
-server <rpc server URL or stratum IP>
-port <rpc port>  [only used for stratum]
-user <username>
-pass <password>
-diff <initial difficulty>  [optional]
-wallet <wallet address>   [only used for solo]
-miner <miner params>

<miner params> format:
[CPU|GPU],<cores or compute units>[<work size>,<platform id>,<device id>[,<loops>]]
<work size>, <platform id> and <device id> are not required for CPU

-hiveos [0|1]   [optional, if 1 will format output for hiveos]
-statrpcurl <URL to send stats to> [optional]
-minername <display name of miner> [required with statrpcurl]
*/

type conf struct {
	NodeUrl        string `yaml:"NodeUrl"`
	NodeUser       string `yaml:"NodeUser"`
	NodePass       string `yaml:"NodePass"`
	WalletAddr     string `yaml:"WalletAddr"`
	StatRpcUrl     string `yaml:"StatRpcUrl"`
	MinerOpts      string `yaml:"MinerOpts"`
	RespawnSeconds int    `yaml:"RespawnSeconds"`
	MinerName      string `yaml:"MinerName"`
}

func (c *conf) getConf() *conf {
	_, err := os.Stat(configFile);

	fmt.Printf("Using config %s\n", configFile)
	yamlFile, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Unable to open config file   #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Fatalf("Config file invalid format: %v", err)
	}

	return c
}

var c conf

func main() {
	var args = os.Args[1:]
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s <config file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Note: This does NOT yet support pool mining or HIVE options\n")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Example:")
		fmt.Fprintf(os.Stderr, "    %s test_miner.yaml\n", os.Args[0])
		os.Exit(1)
	}

	configFile = args[0]

	c.getConf()
	// I NEED TO ADD VALIDATION OF CONFIG OPTIONS AND DEFAULTS!!!


	var timer = c.RespawnSeconds



	ttl = time.Duration(timer) * time.Second
	go func() {
		var minerOn time.Duration
		for {
			startMiner()
			var st = time.Now()
			mineCmd.Wait()
			var dur = time.Since(st)
			minerOn += dur
		}
	}()
	time.Sleep(1)

	for {
		m.Lock()
		if time.Now().After(endMiner) {
			if mineCmd != nil {
				log.Printf("Killing miner")
				mineCmd.Process.Kill()
				mineCmd = nil
			}
		}
		m.Unlock()

		time.Sleep(time.Second * 5)
	}
}

func startMiner() {
	m.Lock()
	defer m.Unlock()

	var now = time.Now()

	endMiner = now.Add(ttl)

	if len(c.StatRpcUrl) > 0 {
		mineCmd = exec.Command("DynMiner2.exe", "-mode", "solo", "-server", c.NodeUrl, "-user", c.NodeUser, "-pass", c.NodePass, "-wallet", c.WalletAddr, "-miner", c.MinerOpts, "-statrpcurl", c.StatRpcUrl, "-minername", c.MinerName)
	} else {
		mineCmd = exec.Command("DynMiner2.exe", "-mode", "solo", "-server", c.NodeUrl, "-user", c.NodeUser, "-pass", c.NodePass, "-wallet", c.WalletAddr, "-miner", c.MinerOpts)
	}

	log.Printf("Executing %q - will end at %s", mineCmd.String(), endMiner)
	time.Sleep(time.Second * 1)
	mineCmd.Stdout = os.Stdout
	mineCmd.Stdout = os.Stdout
	mineCmd.Start()
}
