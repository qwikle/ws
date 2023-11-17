package main

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

func main() {
	ws := NewSocketServer()
			ws.OnConnect(func (socket *Socket) {
			socket.Send("welcome", "Welcome to the server")
			ws.ExceptTo("new_user", "New user joined", []uuid.UUID{socket.Id})
		})
		ws.OnDisconnect(func (socket *Socket) {
			ws.Broadcast("user_left", "User left")
		})

		ws.On("ping", func (data string, socket *Socket) {
			fmt.Print(data) 
			socket.Send("pong", "casse toi de la")
		})

		ws.On("byebye", func (data string, socket *Socket) {
			socket.Send("byebye", "casse toi de la")
			socket.Close()
			fmt.Print(ws.CountSockets())
		})
http.HandleFunc("/ws", ws.HandleWS)
 fmt.Printf("Server started at port 8080\n")

http.HandleFunc("/", func (w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" { 
	ws.Broadcast("new_user", "New Http user")
	w.Write([]byte("Hello world"))
	}
})

 http.ListenAndServe(":8080", nil)

}