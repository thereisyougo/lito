package main

import (
	"flag"
	"lito/web"
	"log"
	"net/http"
	"strconv"
)

func main() {
	// https://meshstudio.io/blog/2017-11-06-serving-html-with-golang/
	localDir := flag.String("dir", ".", "static file server address")
	port := flag.Int("port", 8082, "web server port")
	flag.Parse()

	http.Handle("/", http.RedirectHandler("/jar", http.StatusFound))
	http.HandleFunc("/send", web.AmqpHanler())
	http.HandleFunc("/exec", web.ExecHandler());
	http.HandleFunc("/upload", web.UploadFileHanler())
	http.HandleFunc("/jar", web.UploadJarHanler())
	http.Handle("/files/", http.StripPrefix("/files", http.FileServer(http.Dir(*localDir))))
	log.Printf("Server started on localhost:%v", *port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(*port), nil))

}
