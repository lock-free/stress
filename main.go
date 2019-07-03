package main

import (
	"encoding/json"
	"flag"
	"github.com/lock-free/stress/stress"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func main() {
	// parse args
	configPathPointer := flag.String("config", "./stress_conf.json", "config file path")
	hostPointer := flag.String("host", "", "host variable, like a.com")
	schemePointer := flag.String("scheme", "", "scheme variable, like https")
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// resolve config path
	configPath := filepath.Join(cwd, *configPathPointer)

	// read config
	var stressConfig stress.StressConfig
	err = ReadJson(configPath, &stressConfig)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("[config path] " + configPath)
	stress.StressTesting(stressConfig, *hostPointer, *schemePointer)
}

func ReadJson(filePath string, f interface{}) error {
	source, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(source), f)
}
