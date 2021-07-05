package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/adam000/goutils/shell"
	"github.com/docopt/docopt-go"
	"github.com/gorilla/mux"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Message struct {
	From    string
	Subject string
	Body    string
}

const format = `Subject: %s

From: %s
%s`

const usage = `claptrap-listen

Usage:
	claptrap-listen --web
	claptrap-listen --rabbitmq
`

func main() {
	arguments, err := docopt.ParseDoc(usage)
	if err != nil {
		log.Fatalf("Failed to parse args: %v", err)
	}

	if isWeb, _ := arguments.Bool("--web"); isWeb {
		runWebListener()
	} else {
		runRabbitMqListener()
	}
}

func runRabbitMqListener() {
	username := os.Getenv("RABBITMQ_USERNAME")
	password := os.Getenv("RABBITMQ_PASSWORD")
	host := os.Getenv("RABBITMQ_HOST")
	port := os.Getenv("RABBITMQ_PORT")
	virtualHost := os.Getenv("RABBITMQ_VHOST")
	topic := os.Getenv("RABBITMQ_TOPIC")

	if username == "" {
		log.Fatalf("Username can't be blank; no environment variable found")
	}
	if host == "" {
		log.Fatalf("Host can't be blank; no environment variable found")
	}
	if port == "" {
		log.Fatalf("Port can't be blank; no environment variable found")
	}
	if virtualHost == "" {
		log.Fatalf("Virtual host can't be blank; no environment variable found")
	}
	if topic == "" {
		log.Fatalf("Topic can't be blank; no environment variable found")
	}

	connString := fmt.Sprintf("amqp://%s:%s@%s:%s//%s", username, password, host, port, virtualHost)
	conn, err := amqp.Dial(connString)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}

	msgs, err := ch.Consume(
		topic, // queue
		"",    // consumer
		true,  // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	forever := make(chan struct{})

	go func() {
		for d := range msgs {
			sendMessage(d.Body)
		}
	}()

	log.Printf("Waiting for messages. To exit press CTRL+C")
	<-forever
}

func runWebListener() {
	r := mux.NewRouter()

	r.HandleFunc("/send", mainHandler).Methods("PUT")

	http.Handle("/", r)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read body of request: %v", err)
		// TODO I could probably handle this a little better
		return
	}

	sendMessage(bytes)
}

func sendMessage(bytes []byte) {
	message := &Message{}

	if err := json.Unmarshal(bytes, message); err != nil {
		// if message is malformed, send the first 10KB
		maxSize := 10 * 1024
		if len(bytes) > maxSize {
			bytes = bytes[:maxSize]
		}
		message = &Message{
			From:    "Unknown",
			Subject: "claptrap-listen: A malformed message was received",
			Body:    fmt.Sprintf("Error message: %v\nFirst 10KB: %s", err, string(bytes)),
		}
	}

	output := fmt.Sprintf(format, message.Subject, message.From, message.Body)

	// call out to msmtp
	log.Printf("Message received: %s", output)
	stdout, stderr, err := shell.RunInDirWithStdin(".", output, "msmtp", "adamh.zero@gmail.com")
	if err != nil {
		log.Printf("Error in transmission: %v", err)
	}
	if stdout != "" {
		log.Printf("msmtp stdout: %s", stdout)
	}
	if stderr != "" {
		log.Printf("msmtp stderr: %s", stderr)
	}
}
