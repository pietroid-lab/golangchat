# EP03 — Chat Server em Go

Implementação de um chat TCP em Go com servidor, cliente e bot independentes, utilizando goroutines, channels e protocolo JSON Lines.

## Requisitos

- Go instalado localmente.
- Três terminais para executar servidor, cliente e bot.
- Host e porta fixos: `localhost:4444`.

## Estrutura

- `server.go`: servidor TCP, gerenciamento central de conexões e apelidos, broadcast e roteamento privado.
- `client.go`: cliente interativo com os comandos do enunciado.
- `bot.go`: bot independente que inverte mensagens privadas e responde ao remetente.

## Como executar

Abra três terminais na pasta do projeto.

**Terminal 1 — servidor**

```bash
go run server.go
```

**Terminal 2 — cliente**

```bash
go run client.go
```

Escolha um apelido quando solicitado. Para conectar mais usuários, abra outros terminais e execute novamente `go run client.go` com apelidos diferentes.

**Terminal 3 — bot**

```bash
go run bot.go
```

O bot conecta com o apelido `Inversor`.

> Os três arquivos são programas independentes e cada um declara seu próprio `main`. Execute-os individualmente; não use `go run .`.

## Comandos do cliente

| Comando | Ação |
|---|---|
| `\msg <mensagem>` | Envia uma mensagem pública. |
| `\msg @<apelido> <mensagem>` | Envia uma mensagem privada. |
| `\changenick <apelido>` | Solicita a troca de apelido. |
| `\exit` | Encerra a sessão. |

## Protocolo e concorrência

Os programas trocam mensagens JSON, uma por linha (JSON Lines). A primeira mensagem identifica o cliente ou bot e seu apelido. O servidor utiliza um Hub central como único responsável pelo mapa de conexões e pelos apelidos. Os handlers enviam eventos ao Hub por channels, e cada conexão possui um channel de saída com uma goroutine de escrita dedicada.

O broadcast público é encaminhado somente aos usuários. Mensagens privadas são encaminhadas apenas ao destinatário, que pode ser um usuário ou o bot. O bot inverte o texto usando `[]rune`, preservando caracteres Unicode.

## Validação local

Compile cada programa separadamente:

```bash
go build server.go
go build client.go
go build bot.go
```

Também é possível executar `go vet` individualmente:

```bash
go vet server.go
go vet client.go
go vet bot.go
```

Faça a demonstração com os três processos em execução, incluindo broadcast, mensagem privada para outro usuário, mensagem privada para `Inversor`, alteração de apelido e saída.

## Apresentação

A apresentação com a explicação detalhada da implementação está disponível neste repositório: [apresentacao_ep03.pptx](./apresentacao_ep03.pptx).

Revise e personalize os slides antes da entrega.

## Integridade acadêmica

Revise o código, entenda as decisões de implementação e siga as políticas de uso de ferramentas generativas de IA e de antiplágio da disciplina. Registre sua própria contribuição e não apresente material gerado como autoria independente.
