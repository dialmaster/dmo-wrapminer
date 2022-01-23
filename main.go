package main

import (
	"gopkg.in/yaml.v2"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
//	"strconv"
//	"strings"
	"sync"
	"time"
)

var m sync.Mutex
var mineCmd *exec.Cmd
var minerName string
var minerArgs string
var ttl time.Duration
var endMiner, endChkBlk time.Time
var user, pass, gpu, globalUnits, localUnits, GPUID, walletaddr string
var u *url.URL

/* DynMiner2.exe args:
-mode [solo|stratum]
-server <rpc server URL or stratum IP>
-port <rpc port>  [only used for stratum]
-user <username>
-pass <password>
-diff <initial difficulty>  [optional]
-wallet <wallet address>   [only used for solo]
-miner <miner params>

printf("<miner params> format:\n");
    printf("  [CPU|GPU],<cores or compute units>[<work size>,<platform id>,<device id>[,<loops>]]\n");
    printf("  <work size>, <platform id> and <device id> are not required for CPU\n");

    -hiveos [0|1]   [optional, if 1 will format output for hiveos]
-statrpcurl <URL to send stats to> [optional]
-minername <display name of miner> [required with statrpcurl]
*/

/* 
For my initial purposes... let's read some of these args from a file, things that will not change between my miners:
-server 
-user 
-pass
-statrpcurl
-respawn time

And we will ONLY take the following args:
-miner (eg, like GPU,25600,0,0)
-minername
*/


type conf struct {
	NodeUrl             string  `yaml:"NodeUrl"`
	NodeUser           string  `yaml:"NodeUser"`
	NodePass           string  `yaml:"NodePass"`
	WalletAddr           string  `yaml:"WalletAddr"`
	StatRpcUrl             string  `yaml:"StatRpcUrl"`
	RespawnSeconds int     `yaml:"RespawnSeconds"`
}

func (c *conf) getConf() *conf {
	myConfigFile := "dmowrapconfig.yaml"
	if _, err := os.Stat("mydmowrapconfig.yaml"); err == nil {
		myConfigFile = "mydmowrapconfig.yaml"
	}

	yamlFile, err := ioutil.ReadFile(myConfigFile)
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	return c
}

var c conf




/* NOTE: THIS VERSION IS ONLY FOR SOLO AND DOES NOT SUPPORT THE HIVE STUFF */

func main() {
	var args = os.Args[1:]
    c.getConf() 
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <minerargs> <minername>\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Example:")
		fmt.Fprintf(os.Stderr, "    %s GPU,25600,64,0,0 MyMiner\n", os.Args[0])
		os.Exit(1)
	}

	var timer = c.RespawnSeconds

	minerArgs = args[0]
	minerName = args[1]

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

	mineCmd = exec.Command("DynMiner2.exe", "-mode","solo","-server",c.NodeUrl, "-user",c.NodeUser,"-pass", c.NodePass, "-wallet", c.WalletAddr, "-miner", minerArgs, "-statrpcurl", c.StatRpcUrl, "-minername", minerName)

    log.Printf("Executing %q - will end at %s", mineCmd.String(), endMiner)
	time.Sleep(time.Second * 1)
	mineCmd.Stdout = os.Stdout
	mineCmd.Stdout = os.Stdout
	mineCmd.Start()
}


