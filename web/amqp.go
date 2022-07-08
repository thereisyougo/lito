package web

import (
	"bytes"
	"container/list"
	"encoding/json"
	"fmt"
	q "github.com/streadway/amqp"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	addrTemp = `amqp://{{.Username}}:{{.Password}}@{{.Host}}:{{.Port}}/{{.VirtualHost}}`
)

func FailOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

// Msg represents a message from message queue
type Msg struct {
	BarCode string `json:"barCode"`
	Sign    string `json:"sign"`
	Time    string `json:"time"`
}

type QueueConfig struct {
	Username, Password, Host, Port, VirtualHost, ExchangeName, RouteKey string
}

func buidMsg(m *Msg) string {
	m.Time = time.Now().Format(time.RFC3339)
	if buf, err := json.Marshal(*m); err != nil {
		fmt.Errorf("%s", err)
	} else {
		return fmt.Sprintf("%s", buf)
	}
	return "{}"
}

func sendMsg(ls *list.List, c *QueueConfig) {
	t := template.Must(template.New("index").Parse(addrTemp))
	var buf bytes.Buffer
	t.Execute(&buf, *c)

	s := buf.String()
	fmt.Println("---", s)
	conn, e := q.Dial(s)
	FailOnError(e, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, e := conn.Channel()
	FailOnError(e, "Failed to open a channel")
	defer ch.Close()

	for item := ls.Front(); item != nil; item = item.Next() {
		m := (item.Value).(Msg)
		if e := ch.Publish(c.ExchangeName, c.RouteKey, false,
			false, q.Publishing{
				ContentType: "application/json;charset=UTF-8",
				Body:        []byte(buidMsg(&m)),
			}); e != nil {
			log.Fatalf("%s", e)
		}
	}
}

func amqpindex(w http.ResponseWriter, r *http.Request, q *QueueConfig) {

	//userAgent := r.Header.Get("User-Agent")
	//t := AmqpTempate
	//if strings.Contains(userAgent, "MSIE") {
	//	t = WebTemplate
	//}
	renderPage(w, "web/pages/send.html", *q)

}

func renderPage(w http.ResponseWriter, pagePath string, q interface{}) {
	if templ, e := template.ParseFiles(pagePath); e == nil {
		if e := templ.Execute(w, q); e != nil {
			log.Fatal(e)
		}

	} else {
		log.Panic(e)
		RenderError(w, e.Error(), http.StatusInternalServerError)
	}
}

func msgHandler(w http.ResponseWriter, r *http.Request) {
	codes := r.PostFormValue("codes")
	typ := r.PostFormValue("type")

	//r.PostFormValue()
	queueConfig := QueueConfig{
		r.PostFormValue("username"),
		r.PostFormValue("password"),
		r.PostFormValue("host"),
		r.PostFormValue("port"),
		r.PostFormValue("virtual_host"),
		r.PostFormValue("exchange_name"),
		r.PostFormValue("route_key"),
	}

	fmt.Println(typ)
	if codes != "" {
		if preg, e := regexp.Compile("\\s+"); e != nil {
			log.Fatalf("%s", e)
		} else {
			ls := list.New()

			strArr := preg.Split(codes, -1)
			for _, s := range strArr {
				actual := strings.TrimSpace(s)
				if len(actual) > 0 {
					//sendMsg(actual, typ)
					//for i := 1; i < 10000; i++ {
					ls.PushBack(Msg{
						BarCode: actual,
						Sign:    typ,
					})
					//}
				}
			}

			if ls.Len() > 0 {
				sendMsg(ls, &queueConfig)
			}
		}
	}

	buf, e := json.Marshal(struct {
		Success bool `json:"success"`
	}{true})
	FailOnError(e, "")

	w.Write(buf)
}

func AmqpHanler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.Method
		log.Println(method)
		switch method {
		case "GET":
			q := &QueueConfig{
				Username:     "aas",
				Password:     "aas123",
				Host:         "10.17.2.113",
				Port:         "5672",
				VirtualHost:  "netloan",
				ExchangeName: "netloan-entrance",
				RouteKey:     "#",
			}
			amqpindex(w, r, q)
		case "POST":
			msgHandler(w, r)
		}
	})
}
