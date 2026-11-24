package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
)

const serverAddress = "localhost:4444"

type protocolMessage struct {
	Type string `json:"type"`
	Nick string `json:"nick,omitempty"`
	Kind string `json:"kind,omitempty"`
	To   string `json:"to,omitempty"`
	From string `json:"from,omitempty"`
	Text string `json:"text,omitempty"`
}

func send(encoder *json.Encoder, message protocolMessage) error {
	return encoder.Encode(message)
}

func printServerMessage(message protocolMessage) {
	switch message.Type {
	case "system":
		fmt.Println(message.Text)
	case "public":
		fmt.Printf("@%s disse: %s\n", message.From, message.Text)
	case "private":
		fmt.Printf("@%s disse em privado: %s\n", message.From, message.Text)
	case "error":
		fmt.Printf("Erro: %s\n", message.Text)
	}
}

func handleCommand(line string, encoder *json.Encoder) (shouldExit bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}

	switch {
	case line == `\exit`:
		_ = send(encoder, protocolMessage{Type: "exit"})
		return true

	case strings.HasPrefix(line, `\changenick `):
		nick := strings.TrimSpace(strings.TrimPrefix(line, `\changenick `))
		if nick == "" {
			fmt.Println("Uso: \\changenick <apelido>")
			return false
		}
		if err := send(encoder, protocolMessage{Type: "rename", Nick: nick}); err != nil {
			fmt.Printf("Erro enviando comando: %v\n", err)
			return true
		}

	case strings.HasPrefix(line, `\msg `):
		body := strings.TrimSpace(strings.TrimPrefix(line, `\msg `))
		if body == "" {
			fmt.Println("Uso: \\msg <mensagem> ou \\msg @<apelido> <mensagem>")
			return false
		}

		fields := strings.Fields(body)
		if strings.HasPrefix(fields[0], "@") {
			if len(fields) < 2 {
				fmt.Println("Uso: \\msg @<apelido> <mensagem>")
				return false
			}
			to := strings.TrimPrefix(fields[0], "@")
			text := strings.TrimSpace(body[len(fields[0]):])
			if to == "" || text == "" {
				fmt.Println("Uso: \\msg @<apelido> <mensagem>")
				return false
			}
			if err := send(encoder, protocolMessage{Type: "private", To: to, Text: text}); err != nil {
				fmt.Printf("Erro enviando mensagem: %v\n", err)
				return true
			}
		} else {
			if err := send(encoder, protocolMessage{Type: "public", Text: body}); err != nil {
				fmt.Printf("Erro enviando mensagem: %v\n", err)
				return true
			}
		}

	default:
		fmt.Println("Comando não reconhecido. Use \\msg, \\changenick ou \\exit.")
	}
	return false
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Escolha seu apelido: ")
	nick, err := reader.ReadString('\n')
	if err != nil && len(nick) == 0 {
		fmt.Println("Não foi possível ler o apelido.")
		return
	}
	nick = strings.TrimSpace(nick)
	if nick == "" {
		fmt.Println("O apelido não pode ficar vazio.")
		return
	}

	conn, err := net.Dial("tcp", serverAddress)
	if err != nil {
		fmt.Printf("Não foi possível conectar ao servidor: %v\n", err)
		return
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	if err := send(encoder, protocolMessage{Type: "hello", Nick: nick, Kind: "user"}); err != nil {
		fmt.Printf("Erro enviando identificação: %v\n", err)
		return
	}

	// A goroutine de leitura recebe eventos do servidor enquanto a principal
	// goroutine aguarda os comandos digitados pelo usuário.
	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(conn)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		for scanner.Scan() {
			var message protocolMessage
			if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
				fmt.Println("Recebida uma mensagem inválida do servidor.")
				continue
			}
			printServerMessage(message)
		}
	}()

	fmt.Println("Conectado. Comandos: \\msg <mensagem>, \\msg @apelido <mensagem>, \\changenick <apelido>, \\exit")
	input := make(chan string)
	go func() {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			input <- scanner.Text()
		}
		close(input)
	}()

	for {
		select {
		case line, ok := <-input:
			if !ok || handleCommand(line, encoder) {
				return
			}
		case <-done:
			fmt.Println("O servidor encerrou a conexão.")
			return
		}
	}
}
