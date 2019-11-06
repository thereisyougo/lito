package web

import (
	"context"
	"crypto/md5"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var upgrader = websocket.Upgrader{}

func WsHandler(msgch *chan string, cm *Cmap) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := createSessionId()
		c, err := upgrader.Upgrade(w, r, http.Header{
			"g_sessionid": []string{id},
		})
		cm.set(id, c)
		if err != nil {
			log.Print("upgrade:", err)

		}
		defer func() {
			e := c.Close()
			failOnError(e, "")
		}()

		c.SetCloseHandler(func(code int, text string) error {
			delete(cm.M, id)
			fmt.Println(code, text)
			return nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			for {
				select {
				case m := <-*msgch:
					for _, v := range cm.M {
						//fmt.Println("###" + k)
						err = v.WriteMessage(websocket.TextMessage, []byte(m))
						failOnError(err, "websocket failed")
					}
				case <-ctx.Done():
					return
				}
			}
		}()

		for {
			msgType, p, err := c.ReadMessage()
			//fmt.Println("messageType: ", msgType)
			if err != nil {
				fmt.Println("read message from client -> ", err)
				cancel()
				return
			} else {
				if msgType == websocket.TextMessage {
					*msgch <- string(p)
				}
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

func createSessionId() string {
	nano := time.Now().UnixNano()
	rand.Seed(nano)
	return md5hash(md5hash(strconv.FormatInt(nano, 10)) + md5hash(strconv.FormatInt(rand.Int63(), 10)))
}

func md5hash(text string) string {
	hashMd5 := md5.New()
	_, err := io.WriteString(hashMd5, text)
	failOnError(err, "")
	return fmt.Sprintf("%x", hashMd5.Sum(nil))
}

type Cmap struct {
	M map[string]*websocket.Conn
	L sync.RWMutex
}

func (m *Cmap) set(key string, value *websocket.Conn) {
	m.L.Lock()
	defer m.L.Unlock()
	m.M[key] = value
}

func (m *Cmap) get(key string) *websocket.Conn {
	m.L.RLock()
	defer m.L.RUnlock()
	return m.M[key]
}
