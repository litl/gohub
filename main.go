package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
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

		project := hook.Repo[strings.LastIndex(hook.Repo, "/")+1:]
		if strings.HasPrefix(data.Ref, "refs/tags/") && !data.Deleted {
			go executeShell(hook.Shell, data.Repository.FullName, project, hook.Branch, "tag", data.Ref[10:])
		} else if data.Ref == "refs/heads/"+hook.Branch && !data.Deleted {
			go executeShell(hook.Shell, data.Repository.FullName, project, hook.Branch, "push", data.After)
		} else {
			log.Printf("Unhandled webhook for %s branch %s.  Got:\n%s", data.Repository.FullName,
				hook.Branch, string(body))
		}
	})
}

func executeShell(shell string, args ...string) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	jobId := r.Uint32()

	prefix := fmt.Sprintf("repo=%s jobId=%s ", args[0], strconv.FormatInt(int64(jobId), 10))

	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	logStreamerOut := NewLogstreamer(logger, prefix, false)

	logStreamerOut.Write([]byte(fmt.Sprintf("Running %s %s\n", shell, strings.Join(args, " "))))
	cmd := exec.Command(shell, args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = logStreamerOut
	cmd.Stderr = logStreamerOut
	/*	var b []byte
		buf := bytes.NewBuffer(b)
		cmd.Stderr = buf
	*/err := cmd.Start()
	if err != nil {
		logger.Println(err)
	}

	if err := cmd.Wait(); err != nil {

		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0

			// This works on both Unix and Windows. Although package
			// syscall is generally platform dependent, WaitStatus is
			// defined for both Unix and Windows and in both cases has
			// an ExitStatus() method with the same signature.
			if _, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				logStreamerOut.Write([]byte(fmt.Sprintf("Command finished with error: %v\n", err)))
				return
			}
		} else {
			logStreamerOut.Write([]byte(fmt.Sprintf("Command finished with error: %v\n", err)))
			return
		}
	}
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
