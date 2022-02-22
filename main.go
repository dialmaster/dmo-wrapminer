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
var endMiner time.Time

type mineRpc struct {
	Name        string
	Hashrate    int
	HashrateStr string
	Accept      int
	Reject      int
	Submit      int
}

var accumStats mineRpc
var lastStats mineRpc

/* DynMiner2.exe args for reference:
-mode [solo|stratum|pool]
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
-minername <display name of miner> [required with statrpcurl]

Example Pool:
DynMiner3.exe -mode pool -server pool1.dynamocoin.org -port 4567 -user dy1q96w73wf4s4m4yhtzl7xpcwuw3hzsr2tarmssdw -miner CPU,16

Example Stratum:
-mode stratum -server us-east.deepfields.io -port 4234 -user dy1q4r6xahzwc94l872fnsk5aynsy7pqrngk8x8gt0.3090miner -pass d=3 -miner GPU,32768,128,0,0


*/

type conf struct {
	DynMiner       string   `yaml:"DynMiner"`
	Mode           string   `yaml:"Mode"`
	NodeUrl        string   `yaml:"NodeUrl"`
	NodeUser       string   `yaml:"NodeUser"`
	NodePass       string   `yaml:"NodePass"`
	WalletAddr     string   `yaml:"WalletAddr"`
	MinerOpts      []string `yaml:"MinerOpts,flow"`
	RespawnSeconds int      `yaml:"RespawnSeconds"`
	MinerName      string   `yaml:"MinerName"`
	CloudKey       string   `yaml:"CloudKey"`
	PoolServer     string   `yaml:"PoolServer"`
	PoolPort       string   `yaml:"PoolPort"`
	StartingDiff   string   `yaml:"StartingDiff"`
}

//TODO: Add support for yiimp solo like:
// -pass d=3,m=solo

func (myConfig *conf) getConf() *conf {

	fmt.Printf("Using config %s\n", configFile)
	yamlFile, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Unable to open config file  #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, myConfig)
	if err != nil {
		log.Fatalf("Config file invalid format: %v", err)
	}

	if myConfig.Mode != "solo" && myConfig.Mode != "pool" && myConfig.Mode != "stratum" {
		fmt.Fprintf(os.Stderr, "Mode option from config MUST be one of: solo, pool or stratum\n")
		os.Exit(1)
	}

	_, err = os.Stat(myConfig.DynMiner)
	if err != nil {
		fmt.Fprintf(os.Stderr, "DynMiner: '%s' not found.\n", myConfig.DynMiner)
		os.Exit(1)
	}

	return myConfig
}

var myConfig conf

func main() {
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = ioutil.Discard
	router := gin.Default()

	accumStats.Accept = 0
	lastStats.Accept = 0
	accumStats.Reject = 0
	lastStats.Reject = 0
	accumStats.Submit = 0
	lastStats.Submit = 0

	var args = os.Args[1:]
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s <config file>\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Example:")
		fmt.Fprintf(os.Stderr, "    %s test_miner.yaml\n", os.Args[0])
		os.Exit(1)
	}

	configFile = args[0]

	myConfig.getConf()

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

	go func() {
		time.Sleep(2 * time.Second)

		for {
			m.Lock()
			if time.Now().After(endMiner) {
				if mineCmd != nil {
					mineCmd.Process.Kill()
					mineCmd = nil
				}
			}
			m.Unlock()

			time.Sleep(time.Second * 5)
		}
	}()

	if len(myConfig.CloudKey) > 0 && myConfig.CloudKey != "SOME_CLOUD_KEY" {
		router.POST("/forwardminerstats", forwardMinerStatsRPC)
		router.Run(":18419")
	} else {
		for {
			time.Sleep(100 * time.Millisecond)
		}
	}

}

// Accept stat request from miner, add cloud key, passthrough to dmo-monitor.. maybe do other stuff
func forwardMinerStatsRPC(c *gin.Context) {
	var thisStat mineRpc
	if err := c.BindJSON(&thisStat); err != nil {
		fmt.Printf("Error decoding JSON from miner: %s", err.Error())
		return
	}

	lastStats = thisStat

	thisStat.Accept += accumStats.Accept
	thisStat.Submit += accumStats.Submit
	thisStat.Reject += accumStats.Reject

	var urlString = ""
	if len(myConfig.CloudKey) > 0 && myConfig.CloudKey != "SOME_CLOUD_KEY" {
		reqUrl := url.URL{
			Scheme: "https",
			// This will, in the end, be pointed at dmo-monitor.com, but for now point at my own monitor
			Host: "dmo-monitor.com",
			Path: "minerstats",
		}
		urlString = reqUrl.String()
	}

	payloadBuf := new(bytes.Buffer)
	json.NewEncoder(payloadBuf).Encode(thisStat)
	req, _ := http.NewRequest("POST", urlString, payloadBuf)

	if len(myConfig.CloudKey) > 0 {
		var bearer = "Bearer " + myConfig.CloudKey
		req.Header.Add("Authorization", bearer)
	}

	client := &http.Client{}
	res, e := client.Do(req)
	if e != nil {
		fmt.Printf("Unable to forward request from miner to monitor: %s\n", e.Error())
		return
	}

	defer res.Body.Close()
}

func startMiner() {
	m.Lock()
	defer m.Unlock()

	var now = time.Now()

	endMiner = now.Add(ttl)

	accumStats.Accept += lastStats.Accept
	accumStats.Reject += lastStats.Reject
	accumStats.Submit += lastStats.Submit

	minerArgs := make([]string, 0)
	minerArgs = append(minerArgs, "-mode", myConfig.Mode)

	if myConfig.Mode == "solo" {
		minerArgs = append(minerArgs, "-server", myConfig.NodeUrl)
		minerArgs = append(minerArgs, "-user", myConfig.NodeUser)
		minerArgs = append(minerArgs, "-pass", myConfig.NodePass)
		minerArgs = append(minerArgs, "-wallet", myConfig.WalletAddr)
	}

	if myConfig.Mode == "pool" || myConfig.Mode == "stratum" {
		minerArgs = append(minerArgs, "-server", myConfig.PoolServer)
		minerArgs = append(minerArgs, "-port", myConfig.PoolPort)
	}

	//DynMiner3.exe -mode pool -server pool1.dynamocoin.org -port 4567 -user dy1q96w73wf4s4m4yhtzl7xpcwuw3hzsr2tarmssdw -miner CPU,16
	if myConfig.Mode == "pool" {
		minerArgs = append(minerArgs, "-user", myConfig.WalletAddr)
	}

	//-mode stratum -server us-east.deepfields.io -port 4234 -user dy1q4r6xahzwc94l872fnsk5aynsy7pqrngk8x8gt0.3090miner -pass d=3 -miner GPU,32768,128,0,0
	if myConfig.Mode == "stratum" {
		minerArgs = append(minerArgs, "-user", myConfig.WalletAddr+"."+myConfig.MinerName)
		minerArgs = append(minerArgs, "-pass", "-d="+myConfig.StartingDiff)
	}

	// Used for any miner mode...
	for _, opts := range myConfig.MinerOpts {
		minerArgs = append(minerArgs, "-miner", opts)
	}

	// DMO Monitor support
	if len(myConfig.CloudKey) > 0 && myConfig.CloudKey != "SOME_CLOUD_KEY" {
		minerArgs = append(minerArgs, "-statrpcurl", "http://localhost:18419/forwardminerstats")
		minerArgs = append(minerArgs, "-minername", myConfig.MinerName)
	}

	mineCmd = exec.Command(myConfig.DynMiner, minerArgs...)

	log.Printf("Executing %q - will end at %s", mineCmd.String(), endMiner)
	time.Sleep(time.Second * 1)
	mineCmd.Stdout = os.Stdout
	mineCmd.Stdout = os.Stdout
	mineCmd.Start()
}
