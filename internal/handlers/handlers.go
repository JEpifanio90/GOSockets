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

var views = jet.NewSet(
	jet.NewOSFileSystemLoader("./html"),
	jet.InDevelopmentMode(),
)

var upgradeConnection = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Home renders the home page
func Home(w http.ResponseWriter, r *http.Request) {
	err := renderPage(w, "home.jet", nil)
	if err != nil {
		log.Println(err)
	}
}

type WebSocketConnection struct {
	*websocket.Conn
}

type WSJsonResponse struct {
	Action      string   `json:"action"`
	Message     string   `json:"message"`
	MessageType string   `json:"messageType"`
	Users       []string `json:"users"`
}

type WSPayload struct {
	Action  string              `json:"action"`
	User    string              `json:"user"`
	Message string              `json:"message"`
	Conn    WebSocketConnection `json:"-"`
}

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

	go ListWebSocket(&conn)
}

func ListWebSocket(conn *WebSocketConnection) {
	// Recover from a panic!
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

func ListenToChannel() {
	var res WSJsonResponse
	for {
		event := <-WSChan

		switch event.Action {
		case "username":
			clients[event.Conn] = event.User
			users := getUserList()

			res.Action = "users_list"
			res.Users = users
			BroadcastAll(res)

		case "left":
			// handle the situation where a user leaves the page
			res.Action = "list_users"
			delete(clients, event.Conn)
			users := getUserList()
			res.Users = users
			BroadcastAll(res)

		case "broadcast":
			res.Action = "broadcast"
			res.Message = fmt.Sprintf("<strong>%s</strong>: %s", event.User, event.Message)
			BroadcastAll(res)
		}
	}
}

func BroadcastAll(response WSJsonResponse) {
	for client := range clients {
		err := client.WriteJSON(response)

		if err != nil {
			log.Println("Websocket err")
			_ = client.Close()
			delete(clients, client)
		}
	}
}

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
