package main

import (
	"bytes"
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
	Name     string
	FullName string `json:"full_name"`
}

type GithubJson struct {
	Repository Repository
	Ref        string
	After      string
	Deleted    bool
}

type Config struct {
	Hooks []Hook
}

type Hook struct {
	Repo   string
	Branch string
	Shell  string
}

var config Config

func loadConfig(configFile *string) {
	configData, err := ioutil.ReadFile(*configFile)
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(configData, &config)
	if err != nil {
		log.Fatal(err)
	}

	addHandler()
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

func addHandler() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			return
		}
		defer r.Body.Close()
		decoder := json.NewDecoder(bytes.NewBuffer(body))
		var data GithubJson
		err = decoder.Decode(&data)

		if err != nil {
			log.Println(err)
			return
		}

		var hook Hook
		for _, cfgHook := range config.Hooks {
			if cfgHook.Repo == data.Repository.FullName {
				hook = cfgHook
				break
			}
		}

		if hook.Shell == "" {
			log.Printf("Unhandled webhook for %s branch %s.  Got:\n%s", data.Repository.FullName,
				data.Ref, string(body))
			return
		}

		if strings.HasPrefix(data.Ref, "refs/tags/") && !data.Deleted {
			executeShell(hook.Shell, data.Repository.FullName, hook.Branch, "tag", data.Ref[10:])
		} else if data.Ref == "refs/heads/"+hook.Branch && !data.Deleted {
			executeShell(hook.Shell, data.Repository.FullName, hook.Branch, "push", data.After)
		} else {
			log.Printf("Unhandled webhook for %s branch %s.  Got:\n%s", data.Repository.FullName,
				hook.Branch, string(body))
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
