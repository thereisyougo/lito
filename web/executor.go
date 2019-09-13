package web

import (
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
)

type ExecConfig struct {
	Content, Result string
}

const isWin = runtime.GOOS == "windows"

func ExecHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.Method
		log.Println(method)
		switch method {
		case "GET":
			execIndex(w, r, &ExecConfig{
				Content: "",
				Result:  "",
			})
		case "POST":
			execHandler(w, r)
		}
	})
}

func execHandler(w http.ResponseWriter, r *http.Request) {
	cfg := ExecConfig{
		Content: r.PostFormValue("content"),
	}


	//fields := strings.Fields(content)
	//exec.Command(fields[0], fields[1:]...)
	var cmd *exec.Cmd

	lines := strings.Split(cfg.Content, "\r\n")
	if len(lines) == 1 {
		if isWin {
			cmd = exec.Command("cmd", "/C", lines[0])
		} else {
			cmd = exec.Command("sh", "-c", lines[0])
		}

	} else {
		script := strings.Join(lines, "\n")
		tmpScriptFile, e := CreateTmpScript()
		if e != nil {
			cfg.Result = e.Error()
		}
		tmpScriptFile.WriteString(script)
		if isWin {
			cmd = exec.Command("cmd", tmpScriptFile.Name())
		} else {
			cmd = exec.Command("sh", tmpScriptFile.Name())
		}
	}

	out, e := cmd.CombinedOutput()
	if e != nil {
		cfg.Result = e.Error()
	} else {
		cfg.Result = string(out)
	}

	execIndex(w, r, &cfg)
}

func execIndex(w http.ResponseWriter, r *http.Request, cfg *ExecConfig) {
	renderPage(w, "web/pages/exec.html", *cfg)
}