package web

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

var upgrader = websocket.Upgrader{}

func WsHandler(msgch chan string) (func(w http.ResponseWriter, r *http.Request)) {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)

		}
		defer c.Close()
		for {
			select {
			case m := <-msgch:
				err = c.WriteMessage(websocket.TextMessage, []byte(m))
				failOnError(err, "websocket failed")
			}

			/*mt, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}
			log.Printf("recv: %s", message)
			err = c.WriteMessage(mt, message)
			if err != nil {
				log.Println("write:", err)
				break
			}*/
		}
	}
}
