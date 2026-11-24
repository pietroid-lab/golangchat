package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
)

const serverAddress = "localhost:4444"
const botNickname = "Inversor"

type protocolMessage struct {
	Type string `json:"type"`
	Nick string `json:"nick,omitempty"`
	Kind string `json:"kind,omitempty"`
	To   string `json:"to,omitempty"`
	From string `json:"from,omitempty"`
	Text string `json:"text,omitempty"`
}

func reverseText(text string) string {
	runes := []rune(text) // Runes preserve accented letters and other Unicode characters.
	for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
		runes[left], runes[right] = runes[right], runes[left]
	}
	return string(runes)
}

func main() {
	conn, err := net.Dial("tcp", serverAddress)
	if err != nil {
		fmt.Printf("Não foi possível conectar ao servidor: %v\n", err)
		return
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(protocolMessage{Type: "hello", Nick: botNickname, Kind: "bot"}); err != nil {
		fmt.Printf("Erro enviando identificação do bot: %v\n", err)
		return
	}

	fmt.Printf("Bot @%s conectado. Aguardando mensagens privadas...\n", botNickname)
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var message protocolMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			fmt.Printf("Mensagem inválida recebida: %v\n", err)
			continue
		}

		// The server sends private events to bots, not public chat messages.
		if message.Type == "error" {
			fmt.Printf("Erro do servidor: %s\n", message.Text)
			continue
		}
		if message.Type != "private" {
			continue
		}

		answer := reverseText(message.Text)
		fmt.Printf("Mensagem de: @%s\n", message.From)
		fmt.Printf("Recebi: %s\n", message.Text)
		fmt.Printf("Resposta: %s\n", answer)

		if err := encoder.Encode(protocolMessage{
			Type: "private",
			To:   message.From,
			Text: answer,
		}); err != nil {
			fmt.Printf("Erro enviando resposta: %v\n", err)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Erro lendo mensagens do servidor: %v\n", err)
	}
}
