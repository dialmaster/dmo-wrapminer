package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/gin-gonic/gin"
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
	CloudKey       string `yaml:"CloudKey"`
}

func (myConfig *conf) getConf() *conf {
	_, err := os.Stat(configFile)

	fmt.Printf("Using config %s\n", configFile)
	yamlFile, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Unable to open config file   #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, myConfig)
	if err != nil {
		log.Fatalf("Config file invalid format: %v", err)
	}

	return myConfig
}

var myConfig conf

func main() {
	//gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

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

	myConfig.getConf()
	// I NEED TO ADD VALIDATION OF CONFIG OPTIONS AND DEFAULTS!!!
	var timer = myConfig.RespawnSeconds
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
	time.Sleep(1 * time.Second)

	// Kill the miner every whatever seconds....
	go func() {

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
	}()

	router.POST("/forwardminerstats", forwardMinerStatsRPC)
	router.Run(":18419")

}

type mineRpc struct {
	Name        string
	Hashrate    int
	HashrateStr string
	Accept      int
	Reject      int
	Submit      int
	CloudKey    string
}

// Accept stat request from miner, add cloud key, passthrough to dmo-monitor.. maybe do other stuff
func forwardMinerStatsRPC(c *gin.Context) {
	var thisStat mineRpc
	if err := c.BindJSON(&thisStat); err != nil {
		fmt.Printf("Got unhandled (bad) request!")
		return
	}

	fmt.Printf("Got incoming stats from miner, passing on to monitor: %v", thisStat)
	thisStat.CloudKey = "Testing123Dial"

	// Cloud Key and Local Monitor are mutually exclusive settings...
	urlString := myConfig.StatRpcUrl
	if len(myConfig.CloudKey) > 0 {
		reqUrl := url.URL{
			Scheme: "http",
			// This will, in the end, be pointed at dmo-monitor.com, but for now point at my own monitor
			Host: "192.168.1.174:11235",
			Path: "minerstats",
		}
		urlString = reqUrl.String()
	}

	payloadBuf := new(bytes.Buffer)
	json.NewEncoder(payloadBuf).Encode(thisStat)
	req, _ := http.NewRequest("POST", urlString, payloadBuf)

	client := &http.Client{}
	res, e := client.Do(req)
	if e != nil {
		fmt.Printf("Some error trying to pass through to monitor...\n")
		return
	}

	defer res.Body.Close()

	fmt.Println("response Status:", res.Status)

}

func startMiner() {
	m.Lock()
	defer m.Unlock()

	var now = time.Now()

	endMiner = now.Add(ttl)

	// New way
	minerArgs := make([]string, 0)
	minerArgs = append(minerArgs, "-mode", "solo")
	minerArgs = append(minerArgs, "-server", myConfig.NodeUrl)
	minerArgs = append(minerArgs, "-user", myConfig.NodeUser)
	minerArgs = append(minerArgs, "-pass", myConfig.NodePass)
	minerArgs = append(minerArgs, "-wallet", myConfig.WalletAddr)
	minerArgs = append(minerArgs, "-miner", myConfig.MinerOpts)
	if len(myConfig.CloudKey) > 0 {
		minerArgs = append(minerArgs, "-statrpcurl", "http://localhost:18419/forwardminerstats")
	} else if len(myConfig.StatRpcUrl) > 0 {
		minerArgs = append(minerArgs, "-statrpcurl", myConfig.StatRpcUrl+"minerstats")
	}
	minerArgs = append(minerArgs, "-minername", myConfig.MinerName)
	mineCmd = exec.Command("DynMiner2.exe", minerArgs...)

	log.Printf("Executing %q - will end at %s", mineCmd.String(), endMiner)
	time.Sleep(time.Second * 1)
	mineCmd.Stdout = os.Stdout
	mineCmd.Stdout = os.Stdout
	mineCmd.Start()
}
