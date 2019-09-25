package main

import (
	"flag"
	"fmt"
	"lito/web"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
)

var localDir = flag.String("dir", ".", "static file server address")
var port = flag.Int("port", 8082, "web server port")
var godaemon = flag.Bool("d", false, "run app as a daemon with -d")

func init() {
	if !flag.Parsed() {
		flag.Parse()
	}
	if *godaemon {
		args := os.Args[1:]
		i := 0
		for ; i < len(args); i++ {
			if args[i] == "-d" {
				args = append(args[:i], args[i+1:]...)
				break
			}
		}
		//fmt.Printf("%v", args)
		cmd := exec.Command(os.Args[0], args...)
		cmd.Start()
		fmt.Println("[PID]", cmd.Process.Pid)
		os.Exit(0)
	}
}

func main() {
	// https://meshstudio.io/blog/2017-11-06-serving-html-with-golang/
	if !flag.Parsed() {
		flag.Parse()
	}

	msgch := make(chan string)

	http.Handle("/", http.RedirectHandler("/upload", http.StatusFound))
	http.HandleFunc("/ws", web.WsHandler(msgch))
	http.HandleFunc("/send", web.AmqpHanler())
	http.HandleFunc("/exec", web.ExecHandler())
	http.HandleFunc("/upload", web.UploadFileHanler())
	http.HandleFunc("/jar", web.UploadJarHanler(&msgch))
	http.Handle("/files/", http.StripPrefix("/files", http.FileServer(http.Dir(*localDir))))
	log.Printf("Server started on localhost:%v", *port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(*port), nil))

}
