package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	gubrak "github.com/novalagung/gubrak/v2"
)

type M map[string]interface{}

// MESSAGE_NEW_USER : Ketika ada user baru terhubung ke room, maka sebuah pesan User XXX: connected akan muncul. Konstanta ini digunakan oleh socket server dalam mem-broadcast informasi tersebut.
const MESSAGE_NEW_USER = "New User"

// MESSAGE_CHAT : Ketika user/client mengirim message/pesan ke socket server, message tersebut kemudian diteruskan ke semua client lainnya oleh socket server. Isi pesan di-broadcast oleh socket server ke semua user yang terhubung menggunakan konstanta ini.
const MESSAGE_CHAT = "Chat"

// MESSAGE_LEAVE : Digunakan oleh socket server untuk menginformasikan semua client lainnya, bahwasanya ada client yang keluar dari room (terputus dengan socket server). Pesan User XXX: disconnected dimunculkan.
const MESSAGE_LEAVE = "Leave"

// connections : variabel ini digunakan untuk menampung semua client yang terhubung ke socket server
var connections = make([]*WebSocketConnection, 0)

// SocketPayload : menampung payload yang dikirim dari front end.
type SocketPayload struct {
	Message string
}

// SocketResponse : digunakan oleh back end (socket server) sewaktu mem-broadcast message ke semua client yang terhubun
type SocketResponse struct {
	From    string
	Type    string // berisi salah satu dari konstanta dengan prefix MESSAGE_*
	Message string
}

// WebSocketConnection : client yang terhubung, objek koneksi-nya disimpan pada struct ini
type WebSocketConnection struct {
	*websocket.Conn
	Username string
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		content, err := ioutil.ReadFile("index.html")
		if err != nil {
			http.Error(w, "Could not open requested file", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "%s", content)
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// currentGorillaConn : proses untuk konversi koneksi HTTP ke koneksi web socket
		currentGorillaConn, err := websocket.Upgrade(w, r, w.Header(), 1024, 1024)
		if err != nil {
			http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
		}

		username := r.URL.Query().Get("username")
		currentConn := WebSocketConnection{Conn: currentGorillaConn, Username: username}
		connections = append(connections, &currentConn)

		go handleIO(&currentConn, connections)
	})

	fmt.Println("Server starting at :8080")
	http.ListenAndServe(":8080", nil)
}

func handleIO(currentConn *WebSocketConnection, connections []*WebSocketConnection) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("ERROR", fmt.Sprintf("%v", r))
		}
	}()

	broadcastMessage(currentConn, MESSAGE_NEW_USER, "")

	for {
		payload := SocketPayload{}
		err := currentConn.ReadJSON(&payload)
		if err != nil {
			if strings.Contains(err.Error(), "websocket: close") {
				broadcastMessage(currentConn, MESSAGE_LEAVE, "")
				ejectConnection(currentConn)
				return
			}

			log.Println("ERROR", err.Error())
			continue
		}

		broadcastMessage(currentConn, MESSAGE_CHAT, payload.Message)
	}
}

func ejectConnection(currentConn *WebSocketConnection) {
	filtered := gubrak.From(connections).Reject(func(each *WebSocketConnection) bool {
		return each == currentConn
	}).Result()
	connections = filtered.([]*WebSocketConnection)
}

func broadcastMessage(currentConn *WebSocketConnection, kind, message string) {
	for _, eachConn := range connections {
		if eachConn == currentConn {
			continue
		}

		eachConn.WriteJSON(SocketResponse{
			From:    currentConn.Username,
			Type:    kind,
			Message: message,
		})
	}
}
