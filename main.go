package main

import (
	"fmt"
	"net/http"
)

func main() {
	ws := NewSocketServer()
			ws.OnConnect(func (socket *Socket) {
			fmt.Printf("%d\n",ws.CountSockets())
		})
		ws.OnDisconnect(func (reason string) {
			fmt.Printf("%d\n",ws.CountSockets())
			fmt.Printf("%s\n",reason)
		})

		ws.On("leave", func (data string, socket *Socket) {
			socket.Send("leave", "See you soon")
			socket.Close()
		})

http.HandleFunc("/ws", ws.HandleWS)
 fmt.Printf("Server started at port 8080\n")


 http.ListenAndServe(":8080", nil)
 
}