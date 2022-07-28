package handlers

import (
	"fmt"
	"github.com/CloudyKit/jet/v6"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sort"
)

var WSChan = make(chan WSPayload)

var clients = make(map[WebSocketConnection]string)

// views is the jet view set
var views = jet.NewSet(
	jet.NewOSFileSystemLoader("./html"),
	jet.InDevelopmentMode(),
)

// upgradeConnection is the websocket upgrader from gorilla/websockets
var upgradeConnection = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Home renders the home page
func Home(w http.ResponseWriter, r *http.Request) {
	err := renderPage(w, "home.jet", nil)
	if err != nil {
		log.Println(err)
	}
}

// WebSocketConnection is a wrapper for our websocket connection, in case
// we ever need to put more data into the struct
type WebSocketConnection struct {
	*websocket.Conn
}

// WSJsonResponse defines the response sent back from websocket
type WSJsonResponse struct {
	Action      string   `json:"action"`
	Message     string   `json:"message"`
	MessageType string   `json:"message_type"`
	Users       []string `json:"users"`
}

// WSPayload defines the websocket request from the client
type WSPayload struct {
	Action   string              `json:"action"`
	Username string              `json:"username"`
	Message  string              `json:"message"`
	Conn     WebSocketConnection `json:"-"`
}

// WSEndpoint upgrades connection to websocket
func WSEndpoint(w http.ResponseWriter, r *http.Request) {
	ws, err := upgradeConnection.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}

	log.Println("Client connected to endpoint")

	var response WSJsonResponse
	response.Message = `<em><small>Connected to server</small></em>`

	conn := WebSocketConnection{Conn: ws}
	clients[conn] = ""

	err = ws.WriteJSON(response)
	if err != nil {
		log.Println(err)
	}

	go ListenWS(&conn)
}

// ListenWS is a goroutine that handles communication between server and client, and
// feeds data into the WSChan
func ListenWS(conn *WebSocketConnection) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Error", fmt.Sprintf("%v", r))
		}
	}()

	var payload WSPayload

	for {
		err := conn.ReadJSON(&payload)
		if err == nil {
			payload.Conn = *conn
			WSChan <- payload
		}
	}
}

// ListenToChannel is a goroutine that waits for an entry on the WSChan, and handles it according to the
// specified action
func ListenToChannel() {
	var response WSJsonResponse

	for {
		event := <-WSChan

		switch event.Action {
		case "username":
			// get a list of all users and send it back via broadcast
			clients[event.Conn] = event.Username
			users := getUserList()
			response.Action = "list_users"
			response.Users = users
			broadcastAll(response)

		case "left":
			// handle the situation where a user leaves the page
			response.Action = "list_users"
			delete(clients, event.Conn)
			users := getUserList()
			response.Users = users
			broadcastAll(response)

		case "broadcast":
			response.Action = "broadcast"
			response.Message = fmt.Sprintf("<strong>%s</strong>: %s", event.Username, event.Message)
			broadcastAll(response)
		}
	}
}

// getUserList returns a slice of strings containing all usernames who are currently online
func getUserList() []string {
	var users []string
	for _, name := range clients {
		if name != "" {
			users = append(users, name)
		}
	}
	sort.Strings(users)
	return users
}

// broadcastAll sends ws response to all connected clients
func broadcastAll(response WSJsonResponse) {
	for client := range clients {
		err := client.WriteJSON(response)
		if err != nil {
			// the user probably left the page, or their connection dropped
			log.Println("websocket err")
			_ = client.Close()
			delete(clients, client)
		}
	}
}

// renderPage renders a jet template
func renderPage(w http.ResponseWriter, tmpl string, data jet.VarMap) error {
	view, err := views.GetTemplate(tmpl)
	if err != nil {
		log.Println(err)
		return err
	}

	err = view.Execute(w, data, nil)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}
