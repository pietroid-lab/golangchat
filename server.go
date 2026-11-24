package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
)

const listenAddress = "localhost:4444"

// protocolMessage is the JSON message exchanged by the server, client and bot.
// Each JSON object is sent on a single line (JSON Lines).
type protocolMessage struct {
	Type string `json:"type"`
	Nick string `json:"nick,omitempty"`
	Kind string `json:"kind,omitempty"`
	To   string `json:"to,omitempty"`
	From string `json:"from,omitempty"`
	Text string `json:"text,omitempty"`
}

type client struct {
	conn net.Conn
	send chan protocolMessage
	nick string // Updated only by the Hub goroutine.
	kind string // "user" or "bot"; updated only by the Hub goroutine.
}

type registerRequest struct {
	client *client
	nick   string
	kind   string
	ack    chan error
}

type incomingRequest struct {
	client  *client
	message protocolMessage
}

type hub struct {
	register   chan registerRequest
	unregister chan *client
	incoming   chan incomingRequest
}

func newHub() *hub {
	return &hub{
		register:   make(chan registerRequest),
		unregister: make(chan *client),
		incoming:   make(chan incomingRequest),
	}
}

func normalizeNick(nick string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(nick), "@"))
}

func validateNick(nick string) (string, error) {
	nick = strings.TrimSpace(strings.TrimPrefix(nick, "@"))
	if nick == "" {
		return "", errors.New("o apelido não pode ficar vazio")
	}
	if len([]rune(nick)) > 24 {
		return "", errors.New("o apelido deve ter no máximo 24 caracteres")
	}
	if strings.ContainsAny(nick, " \t\r\n@") {
		return "", errors.New("o apelido não pode conter espaços nem @")
	}
	return nick, nil
}

func writeMessage(conn net.Conn, message protocolMessage) error {
	return json.NewEncoder(conn).Encode(message)
}

func (c *client) writeLoop() {
	encoder := json.NewEncoder(c.conn)
	for message := range c.send {
		if err := encoder.Encode(message); err != nil {
			_ = c.conn.Close() // Unblocks the reader, which will unregister this client.
			return
		}
	}
}

func sendTo(c *client, message protocolMessage) {
	// A slow client must not block the Hub and stop the entire chat.
	select {
	case c.send <- message:
	default:
		_ = c.conn.Close()
	}
}

func broadcastToUsers(clients map[string]*client, message protocolMessage) {
	for _, c := range clients {
		if c.kind == "user" {
			sendTo(c, message)
		}
	}
}

func (h *hub) run() {
	// The Hub is the only goroutine that reads or writes this map and the
	// nick/kind fields. This avoids needing a mutex for chat state.
	clients := make(map[string]*client)

	for {
		select {
		case request := <-h.register:
			nick, err := validateNick(request.nick)
			if err == nil && request.kind != "user" && request.kind != "bot" {
				err = errors.New("tipo de conexão inválido")
			}
			key := normalizeNick(nick)
			if err == nil {
				if _, exists := clients[key]; exists {
					err = errors.New("esse apelido já está em uso")
				}
			}
			if err != nil {
				request.ack <- err
				continue
			}

			request.client.nick = nick
			request.client.kind = request.kind
			clients[key] = request.client

			var announcement string
			if request.kind == "bot" {
				announcement = fmt.Sprintf("Bot @%s acabou de entrar", nick)
			} else {
				announcement = fmt.Sprintf("Usuário @%s acabou de entrar", nick)
			}
			fmt.Println(announcement)
			broadcastToUsers(clients, protocolMessage{Type: "system", Text: announcement})
			request.ack <- nil

		case request := <-h.incoming:
			c := request.client
			// Ignore messages that arrive after this connection was removed.
			if clients[normalizeNick(c.nick)] != c {
				continue
			}

			switch request.message.Type {
			case "public":
				if c.kind != "user" {
					sendTo(c, protocolMessage{Type: "error", Text: "bots não podem enviar mensagens públicas"})
					continue
				}
				text := strings.TrimSpace(request.message.Text)
				if text == "" {
					continue
				}
				broadcastToUsers(clients, protocolMessage{
					Type: "public",
					From: c.nick,
					Text: text,
				})

			case "private":
				text := strings.TrimSpace(request.message.Text)
				targetNick := strings.TrimPrefix(strings.TrimSpace(request.message.To), "@")
				if targetNick == "" || text == "" {
					sendTo(c, protocolMessage{Type: "error", Text: "destinatário e mensagem são obrigatórios"})
					continue
				}
				target, exists := clients[normalizeNick(targetNick)]
				if !exists {
					sendTo(c, protocolMessage{Type: "error", Text: "usuário ou bot não encontrado: @" + targetNick})
					continue
				}

				fmt.Printf("@%s disse em privado: %s\n", c.nick, text)
				sendTo(target, protocolMessage{
					Type: "private",
					From: c.nick,
					To:   target.nick,
					Text: text,
				})

			case "rename":
				if c.kind != "user" {
					sendTo(c, protocolMessage{Type: "error", Text: "bots não podem trocar de apelido"})
					continue
				}
				newNick, err := validateNick(request.message.Nick)
				if err != nil {
					sendTo(c, protocolMessage{Type: "error", Text: err.Error()})
					continue
				}
				newKey := normalizeNick(newNick)
				if other, exists := clients[newKey]; exists && other != c {
					sendTo(c, protocolMessage{Type: "error", Text: "esse apelido já está em uso"})
					continue
				}

				oldNick := c.nick
				delete(clients, normalizeNick(oldNick))
				c.nick = newNick
				clients[newKey] = c

				announcement := fmt.Sprintf("Usuário @%s agora é @%s", oldNick, newNick)
				fmt.Println(announcement)
				broadcastToUsers(clients, protocolMessage{Type: "system", Text: announcement})

			default:
				sendTo(c, protocolMessage{Type: "error", Text: "mensagem ou comando desconhecido"})
			}

		case c := <-h.unregister:
			key := normalizeNick(c.nick)
			if current, exists := clients[key]; exists && current == c {
				delete(clients, key)
				close(c.send)
				_ = c.conn.Close()
				if c.kind == "user" {
					announcement := fmt.Sprintf("Usuário @%s saiu", c.nick)
					fmt.Println(announcement)
					broadcastToUsers(clients, protocolMessage{Type: "system", Text: announcement})
				}
			} else {
				_ = c.conn.Close()
			}
		}
	}
}

func handleConnection(conn net.Conn, h *hub) {
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), 1024*1024)

	// The first line must identify the nickname and whether this is a user or bot.
	if !scanner.Scan() {
		_ = conn.Close()
		return
	}
	var hello protocolMessage
	if err := json.Unmarshal(scanner.Bytes(), &hello); err != nil || hello.Type != "hello" {
		_ = writeMessage(conn, protocolMessage{Type: "error", Text: "primeira mensagem deve ser hello"})
		_ = conn.Close()
		return
	}

	c := &client{conn: conn, send: make(chan protocolMessage, 64)}
	ack := make(chan error, 1)
	h.register <- registerRequest{client: c, nick: hello.Nick, kind: hello.Kind, ack: ack}
	if err := <-ack; err != nil {
		_ = writeMessage(conn, protocolMessage{Type: "error", Text: err.Error()})
		_ = conn.Close()
		return
	}

	go c.writeLoop()
	defer func() { h.unregister <- c }()

	for scanner.Scan() {
		var message protocolMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			sendTo(c, protocolMessage{Type: "error", Text: "formato de mensagem inválido"})
			continue
		}
		if message.Type == "exit" {
			return
		}
		h.incoming <- incomingRequest{client: c, message: message}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Erro lendo conexão TCP: %v", err)
	}
}

func main() {
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.Fatalf("não foi possível iniciar o servidor em %s: %v", listenAddress, err)
	}
	defer listener.Close()

	fmt.Printf("Servidor iniciado em %s\n", listenAddress)
	h := newHub()
	go h.run()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Erro aceitando conexão: %v", err)
			continue
		}
		go handleConnection(conn, h)
	}
}
