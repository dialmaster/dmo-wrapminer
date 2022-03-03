package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/blang/semver/v4"
)

var wrapMinerFile = "dmo-wrapminer"
var localTesting = false

func main() {

	var args = os.Args[1:]

	_, err := os.Stat(wrapMinerFile)
	if err != nil {
		wrapMinerFile = "dmo-wrapminer.exe"
		_, err2 := os.Stat(wrapMinerFile)
		if err2 != nil {
			fmt.Printf("dmo-wrapminer executable is missing!\n")
			os.Exit(1)
		}
	}

	getNewWrapMiner()

	fmt.Printf("Launcher is executing dmo-wrapminer...\n")
	if len(args) == 0 {
		args = append(args, "NO_CONFIG_FILE")
	}

	args = append(args, "launcher")

	// Run dmo-wrapminer. If it exits with status 69 then download a new one and restart it.. Otherwise just exit.
	for {
		runCmd := exec.Command(wrapMinerFile, args...)

		runCmd.Stdout = os.Stdout

		if err := runCmd.Start(); err != nil {
			log.Fatalf("cmd.Start: %v", err)
		}

		var exitStatus = 0

		if err := runCmd.Wait(); err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
					exitStatus = status.ExitStatus()
					log.Printf("Exit Status: %d", status.ExitStatus())
				}
			} else {
				log.Fatalf("cmd.Wait: %v", err)
			}
		}

		if exitStatus != 69 {
			os.Exit(0)
		}
		getNewWrapMiner()
	}

}

func getNewWrapMiner() {
	versionInfo, err := exec.Command(wrapMinerFile, "version").Output()

	if err != nil {
		fmt.Printf("Unable to execute %s: %s\n", wrapMinerFile, err.Error())
		os.Exit(1)
	}

	versions := strings.Split(string(versionInfo), ",")
	versions[1] = strings.TrimSpace(versions[1])

	myV, _ := semver.Make(versions[0])
	curV, _ := semver.Make(versions[1])
	if curV.LT(myV) {
		return
	}

	fmt.Printf("NOTE: A new dmo-wrapminer version %s is available from https://dmo-monitor.com/wrapminer\n\n", versions[1])
	fmt.Printf("Downloading new version.\n")

	var binURL = "https://dmo-monitor.com/static/dmo-wrapminer/bin/" + wrapMinerFile
	if localTesting {
		binURL = "http://localhost:11235/static/dmo-wrapminer/bin/" + wrapMinerFile
	}

	// Build fileName from fullPath
	fileURL, err := url.Parse(binURL)
	if err != nil {
		fmt.Printf("Unable to update dmo-wrapminer from site: %s", err.Error())
		return
	}
	path := fileURL.Path
	segments := strings.Split(path, "/")
	var fileName = segments[len(segments)-1]

	file, err := os.Create(fileName)
	if err != nil {
		fmt.Printf("Unable to update dmo-wrapminer from site: %s", err.Error())
		return
	}
	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}
	resp, err := client.Get(binURL)
	if err != nil {
		fmt.Printf("Unable to update dmo-wrapminer from site: %s", err.Error())
		return
	}
	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)

	if err != nil {
		fmt.Printf("Unable to update dmo-wrapminer from site: %s", err.Error())
		return
	}

	fmt.Printf("Successfully updated dmo-wrapminer with new version.")
	defer file.Close()
}
