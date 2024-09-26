package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"golang.org/x/net/websocket"
)

type Msg struct {
	clientKey string
	text      string
}

type NewClientEvent struct {
	clientKey string
	msgChan   chan *Msg
}

const MaxBackLog = 100

var dirPath string
var clientRequests = make(chan *NewClientEvent, 100)
var clientDisconnects = make(chan string, 100)
var messages = make(chan *Msg, 100)

func IndexPage(w http.ResponseWriter, r *http.Request) {
	fp, err := os.Open(dirPath + "src/index.html")
	if err != nil {
		log.Println("Could not open file! ", err.Error())
		w.Write([]byte("Could not opened index.html"))
		return
	}

	defer fp.Close()

	_, err = io.Copy(w, fp)
	if err != nil {
		log.Println("Could not send file ", err.Error())
		w.Write([]byte("Could not send file!"))
		return
	}
}

func ChatServer(ws *websocket.Conn) {
	lenBuf := make([]byte, 1024)

	msgChan := make(chan *Msg, 100)
	clientKey := ws.RemoteAddr().String()
	clientRequests <- &NewClientEvent{
		clientKey,
		msgChan,
	}
	defer func() {
		clientDisconnects <- clientKey
	}()

	go func() {
		for msg := range msgChan {
			ws.Write([]byte(msg.text))
		}
	}()

	for {
		_, err := ws.Read(lenBuf)
		if err != nil {
			log.Println("Error: ", err.Error())
			return
		}

		length, _ := strconv.Atoi(strings.TrimSpace(string(lenBuf)))
		if length > 65536 {
			log.Println("Error: too big length: ", lenBuf)
			return
		}


		buf := make([]byte, length)
		_, err = ws.Read(buf)
		if err != nil {
			log.Println("Could not read ", length, " bytes: ", err.Error())
			return
		}

		messages <- &Msg{
			clientKey,
			string(buf),
		}
	}
}

func router() {
	clients := make(map[string]chan *Msg)

	for {
		select {
		case req := <-clientRequests:
			clients[req.clientKey] = req.msgChan
			log.Println("websocket connected: " + req.clientKey)
		case clientKey := <-clientDisconnects:
			close(clients[clientKey])
			delete(clients, clientKey)
			log.Println("websocket disconnected: " + clientKey)
		case msg := <-messages:
			for _, msgChan := range clients {
				if len(msgChan) < cap(msgChan) {
					msgChan <- msg
				}
			}
		}
	}
}

func main() {
	go router()

	http.HandleFunc("/", IndexPage)
	http.Handle("/ws", websocket.Handler(ChatServer))

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
