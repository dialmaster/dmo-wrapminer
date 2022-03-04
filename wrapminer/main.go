package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/blang/semver/v4"

	"gopkg.in/yaml.v2"

	"github.com/denisbrodbeck/machineid"
	"github.com/gin-gonic/gin"
)

var configFile string
var m sync.Mutex
var mineCmd *exec.Cmd
var ttl time.Duration
var endMiner time.Time

type mineRpc struct {
	Name        string
	MinerID     string
	Hashrate    int
	HashrateStr string
	Accept      int
	Reject      int
	Submit      int
	Diff        float64
}

var accumStats mineRpc
var lastStats mineRpc
var myPort = 18419
var myVersion = "1.5.1"
var usedLauncher = 0

var localTesting = false

type conf struct {
	DynMiner             string   `yaml:"DynMiner"`
	Mode                 string   `yaml:"Mode"`
	NodeUrl              string   `yaml:"NodeUrl"`
	NodeUser             string   `yaml:"NodeUser"`
	NodePass             string   `yaml:"NodePass"`
	WalletAddr           string   `yaml:"WalletAddr"`
	MinerOpts            []string `yaml:"MinerOpts,flow"`
	RespawnSeconds       int      `yaml:"RespawnSeconds"`
	MinerName            string   `yaml:"MinerName"`
	CloudKey             string   `yaml:"CloudKey"`
	PoolServer           string   `yaml:"PoolServer"`
	PoolPort             string   `yaml:"PoolPort"`
	StartingDiff         string   `yaml:"StartingDiff"`
	SRBMiner             string   `yaml:"SRBMiner"`
	SRBPoolUrl           string   `yaml:"SRBPoolUrl"`
	SRBMode              string   `yaml:"SRBMode"`
	SRBAdditionalOpts    []string `yaml:"SRBAdditionalOpts,flow"`
	CheckUpdateFrequency int      `yaml:"CheckUpdateFrequency"`
}

//TODO: Add support for yiimp solo like:
// -pass d=3,m=solo

func (myConfig *conf) getConf() *conf {
	if len(configFile) == 0 {
		configFile = "mydmowrapconfig.yaml"
		fmt.Printf("Using default config file: %s\n", configFile)
	} else {
		fmt.Printf("Using specified config file: %s\n", configFile)
	}

	yamlFile, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Unable to open config file  #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, myConfig)
	if err != nil {
		log.Fatalf("Config file invalid format: %v", err)
	}

	validateConfig()

	if len(myConfig.MinerName) > 2 && myConfig.MinerName[len(myConfig.MinerName)-1] == '$' && myConfig.MinerName[0] == '$' {
		envName := myConfig.MinerName[1 : len(myConfig.MinerName)-1]
		envValue, ok := os.LookupEnv(envName)
		if !ok {
			fmt.Printf("WARNING: yaml config specified minerName from ENV but ENV value for %s was not found!\n", envName)
			fmt.Printf("Defaulting miner name to %s\n", envName)
			myConfig.MinerName = envName
		} else {
			fmt.Printf("Using MinerName from ENV: %s\n", envValue)
			myConfig.MinerName = envValue
		}
	}

	if myConfig.CheckUpdateFrequency == 0 {
		myConfig.CheckUpdateFrequency = 720
	}

	return myConfig
}

func validateConfig() {
	if myConfig.Mode != "solo" && myConfig.Mode != "pool" && myConfig.Mode != "stratum" && myConfig.Mode != "SRB" {
		fmt.Fprintf(os.Stderr, "Mode option from config MUST be one of: solo, stratum, or SRB\n")
		os.Exit(1)
	}

	if myConfig.Mode != "SRB" {
		_, err := os.Stat(myConfig.DynMiner)
		if err != nil {
			fmt.Fprintf(os.Stderr, "DynMiner: '%s' not found.\n", myConfig.DynMiner)
			os.Exit(1)
		}
	} else {
		_, err := os.Stat(myConfig.SRBMiner)
		if err != nil {
			fmt.Fprintf(os.Stderr, "SRBMiner: '%s' not found.\n", myConfig.SRBMiner)
			os.Exit(1)
		}
		if myConfig.SRBMode != "GPU" && myConfig.SRBMode != "CPU" {
			fmt.Fprintf(os.Stderr, "SRBMode must be set to either 'CPU' or 'GPU'\n")
			os.Exit(1)
		}
	}

	if len(myConfig.WalletAddr) == 0 {
		fmt.Printf("WalletAddr not set in config. Exiting\n")
		os.Exit(1)

	}

	if len(myConfig.MinerName) == 0 {
		fmt.Printf("MinerName not set in config. Exiting\n")
		os.Exit(1)
	}

}

var myConfig conf

var minerID string

func main() {
	var args = os.Args[1:]
	var siteVersion = checkVersion()

	// For the launcher -- if 'version' is the only arg passed, simply return the current version and site version and exit
	if len(args) == 1 {
		if args[0] == "version" {
			fmt.Printf("%s,%s\n", myVersion, siteVersion)
			os.Exit(0)
		}
	}

	fmt.Printf("Currently running dmo-wrapminer version: %s\n\n", myVersion)

	myV, _ := semver.Make(myVersion)
	curV, _ := semver.Make(siteVersion)
	if myV.LT(curV) {
		fmt.Printf("NOTE: A new dmo-wrapminer version %s is available from https://dmo-monitor.com/wrapminer\n\n", siteVersion)
	}

	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = ioutil.Discard
	router := gin.Default()

	accumStats.Accept = 0
	lastStats.Accept = 0
	accumStats.Reject = 0
	lastStats.Reject = 0
	accumStats.Submit = 0
	lastStats.Submit = 0

	if len(args) > 1 && args[1] != "launcher" {
		fmt.Fprintf(os.Stderr, "Usage: %s <optional config file name>\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Example:")
		fmt.Fprintf(os.Stderr, "    %s test_miner.yaml\n", os.Args[0])
		os.Exit(1)
	}

	// This determines if we check for a new version occasionally and, if we find one, exit with a status that tells
	// the launcher to update and relaunch
	if len(args) == 2 && args[1] == "launcher" {
		usedLauncher = 1
		fmt.Printf("This was run using the launcher. If a new dmo-wrapminer becomes available it will automatically be downloaded and restarted.\n")
	}

	findOpenPort()

	configFile = ""
	if len(args) == 1 {
		configFile = args[0]
	}
	if len(args) == 2 && args[0] != "NO_CONFIG_FILE" {
		configFile = args[0]
	}

	myConfig.getConf()

	machineID, _ := machineid.ProtectedID("dmo-wrapminer")
	minerID = myConfig.MinerName + "-" + strconv.Itoa(myPort) + "-" + machineID

	fmt.Printf("MinerID is: %s\n", minerID)

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
			if time.Now().After(endMiner) && myConfig.RespawnSeconds != 0 {
				if mineCmd != nil {
					mineCmd.Process.Kill()
					mineCmd = nil
				}
			}
			m.Unlock()

			time.Sleep(time.Second * 5)
			if myConfig.Mode == "SRB" && len(myConfig.CloudKey) > 0 && myConfig.CloudKey != "SOME_CLOUD_KEY" {
				getSRBStats()
			}
		}
	}()

	// Auto-update functionality if run from the launcher
	if usedLauncher == 1 {
		go func() {
			fmt.Printf("Will check for new dmo-wrapminer version every %d minutes.\n", myConfig.CheckUpdateFrequency)
			time.Sleep(time.Duration(myConfig.CheckUpdateFrequency) * time.Minute)
			for {
				fmt.Printf("Checking dmo-monitor for new dmo-wrapminer version.\n")
				var siteVersion = checkVersion()
				myV, _ := semver.Make(myVersion)
				curV, _ := semver.Make(siteVersion)
				if myV.LT(curV) {
					fmt.Printf("New dmo-wrapminer version found, updating and relaunching.\n")
					mineCmd.Process.Kill()
					os.Exit(69)
				}
				time.Sleep(time.Duration(myConfig.CheckUpdateFrequency) * time.Minute)
			}
		}()
	}

	if len(myConfig.CloudKey) > 0 && myConfig.CloudKey != "SOME_CLOUD_KEY" {
		router.POST("/forwardminerstats", forwardMinerStatsRPC)
		router.Run(":" + strconv.Itoa(myPort))
	} else {
		for {
			time.Sleep(100 * time.Millisecond)
		}
	}

}

func findOpenPort() {
	portsChecked := 0
	for {
		server, err := net.Listen("tcp", ":"+strconv.Itoa(myPort))
		if err != nil {
			myPort += 1
			portsChecked += 1
		} else {
			fmt.Printf("Using port %d\n", myPort)
			server.Close()
			break
		}
		if portsChecked > 20 {
			fmt.Printf("Unable to find open port to bind!\n")
			os.Exit(1)
		}

	}
}

func checkVersion() string {
	type wrapVersion struct {
		Version string `json:"Version"`
	}

	var curVersion wrapVersion

	var scheme = "https"
	var host = "dmo-monitor.com"
	if localTesting {
		scheme = "http"
		host = "localhost:11235"
	}
	reqUrl := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   "dmowrapversioncheck",
	}
	urlString := reqUrl.String()
	resp, err := http.Get(urlString)

	if err != nil {
		fmt.Printf("Failed request to dmo-monitor for version check: %s", err.Error())
		return "0.0"
	}

	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed request to dmo-monitor for version check: %s", err.Error())
		return "0.0"
	}

	if err := json.Unmarshal(bodyText, &curVersion); err != nil {
		fmt.Printf("Failed request to dmo-monitor for version check: %s", err.Error())
		return "0.0"
	}

	return curVersion.Version
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
	thisStat.MinerID = minerID
	sendMyStatsToMonitor(thisStat)
}

func sendMyStatsToMonitor(thisStat mineRpc) {
	var urlString = ""
	if len(myConfig.CloudKey) > 0 && myConfig.CloudKey != "SOME_CLOUD_KEY" {
		var scheme = "https"
		var host = "dmo-monitor.com"
		if localTesting {
			scheme = "http"
			host = "localhost:11235"
		}

		reqUrl := url.URL{
			Scheme: scheme,
			Host:   host,
			Path:   "minerstats",
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

	if myConfig.Mode != "SRB" {
		mineCmd = setupFoundationMiner()
	} else {
		mineCmd = setupSRBMiner()
	}

	log.Printf("Executing %q - will end at %s", mineCmd.String(), endMiner)
	time.Sleep(time.Second * 1)
	mineCmd.Stdout = os.Stdout
	mineCmd.Stdout = os.Stdout
	mineCmd.Start()
}

func setupSRBMiner() *exec.Cmd {
	minerArgs := make([]string, 0)

	minerArgs = append(minerArgs, "--api-enable")
	minerArgs = append(minerArgs, "--algorithm", "dynamo")
	if myConfig.SRBMode == "GPU" {
		minerArgs = append(minerArgs, "--disable-cpu")
	}

	minerArgs = append(minerArgs, "--pool", myConfig.SRBPoolUrl)
	minerArgs = append(minerArgs, "--wallet", myConfig.WalletAddr)
	minerArgs = append(minerArgs, myConfig.SRBAdditionalOpts...)

	mineCmd = exec.Command(myConfig.SRBMiner, minerArgs...)

	return mineCmd
}

func setupFoundationMiner() *exec.Cmd {
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
		minerArgs = append(minerArgs, "-statrpcurl", "http://localhost:"+strconv.Itoa(myPort)+"/forwardminerstats")
		minerArgs = append(minerArgs, "-minername", myConfig.MinerName)
	}
	mineCmd = exec.Command(myConfig.DynMiner, minerArgs...)
	return mineCmd
}
