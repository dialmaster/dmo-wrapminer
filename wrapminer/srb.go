package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type SRBResponse struct {
	RigName         string `json:"rig_name"`
	MinerVersion    string `json:"miner_version"`
	MiningTime      int    `json:"mining_time"`
	TotalCPUWorkers int    `json:"total_cpu_workers"`
	TotalGpuWorkers int    `json:"total_gpu_workers"`
	TotalWorkers    int    `json:"total_workers"`
	Algorithms      []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Pool struct {
			Difficulty float64 `json:"difficulty"`
		} `json:"pool"`
		Shares struct {
			Total    int `json:"total"`
			Accepted int `json:"accepted"`
			Rejected int `json:"rejected"`
		} `json:"shares"`
		Hashrate struct {
			OneMin float64 `json:"1min"`
			OneHr  float64 `json:"1hr"`
			SixHr  float64 `json:"6hr"`
			One2Hr float64 `json:"12hr"`
		} `json:"hashrate"`
	} `json:"algorithms"`
}

func getSRBStats() {
	var result SRBResponse
	reqUrl := url.URL{
		//Scheme: "http",
		//Host:   "localhost:11235",
		Scheme: "http",
		Host:   "localhost:21550",
		Path:   "",
	}
	urlString := reqUrl.String()
	resp, err := http.Get(urlString)

	if err != nil {
		fmt.Printf("Failed request to SRBMiner: %s", err.Error())
		return
	}

	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed request to SRBMiner: %s", err.Error())
		return
	}

	if err := json.Unmarshal(bodyText, &result); err != nil {
		fmt.Printf("Failed request to SRBMiner: %s", err.Error())
		return
	}
	// Find dynamo data since we don't currently care about/support other algos
	var thisStat mineRpc

	for _, v := range result.Algorithms {
		if v.Name == "dynamo" {
			thisStat.Hashrate = int(v.Hashrate.OneMin)
			thisStat.Submit = v.Shares.Total
			thisStat.Accept = v.Shares.Accepted
			thisStat.Reject = v.Shares.Rejected
			thisStat.Diff = v.Pool.Difficulty
		}
	}

	mutex.Lock()
	lastStats = thisStat
	thisStat.Accept += accumStats.Accept
	thisStat.Submit += accumStats.Submit
	thisStat.Reject += accumStats.Reject
	mutex.Unlock()

	thisStat.MinerID = minerID
	thisStat.Name = myConfig.MinerName

	sendMyStatsToMonitor(thisStat)

}
