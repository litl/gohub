package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

type Repository struct {
	Name string
}

type GithubJson struct {
	Repository Repository
	Ref        string
	After      string
}

type Config struct {
	Hooks []Hook
}

type Hook struct {
	Repo   string
	Branch string
	Shell  string
}

func loadConfig(configFile *string) {
	var config Config
	configData, err := ioutil.ReadFile(*configFile)
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(configData, &config)
	if err != nil {
		log.Fatal(err)
	}
	for i := 0; i < len(config.Hooks); i++ {
		addHandler(config.Hooks[i].Repo, config.Hooks[i].Branch, config.Hooks[i])
	}
}

func setLog(logFile *string) {
	if *logFile != "" {
		log_handler, err := os.OpenFile(*logFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0777)
		if err != nil {
			panic("cannot write log")
		}
		log.SetOutput(log_handler)
	}
	log.SetFlags(5)
}

func startWebserver() {
	log.Printf("Starting gohub on 0.0.0.0:%s", *port)
	http.ListenAndServe(":"+*port, nil)
}

func addHandler(repo, branch string, hook Hook) {
	uri := branch
	branch = "refs/heads/" + branch
	http.HandleFunc("/"+repo+"_"+uri, func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var data GithubJson
		err := decoder.Decode(&data)

		if err != nil {
			log.Println(err)
		}
		if data.Repository.Name == repo && strings.HasPrefix(data.Ref, "refs/tags/") {
			executeShell(hook.Shell, repo, uri, "tag", data.Ref[10:])
		} else if data.Repository.Name == repo && data.Ref == branch {
			executeShell(hook.Shell, repo, uri, "push", data.After)
		} else {
			executeShell(hook.Shell)
		}

	})
}

func executeShell(shell string, args ...string) {
	out, err := exec.Command(shell, args...).Output()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Shell output was: %s\n", out)
}

var (
	port       = flag.String("port", "7654", "port to listen on")
	configFile = flag.String("config", "./config.json", "config")
	logFile    = flag.String("log", "", "log file")
)

func init() {
	flag.Parse()
}

func main() {
	setLog(logFile)
	loadConfig(configFile)
	startWebserver()
}
