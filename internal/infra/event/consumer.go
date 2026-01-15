package event

import (
	"encoding/json"
	"fmt"
	"github.com/DioGolang/GoFleet/internal/application/dto"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"time"
)

type Consumer struct {
	Conn *amqp.Connection
}

func NewConsumer(conn *amqp.Connection) *Consumer {
	return &Consumer{
		Conn: conn,
	}
}

func (c *Consumer) Start(queueName string) error {
	ch, err := c.Conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	msgs, err := ch.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			fmt.Printf("I received a message: %s\n", d.Body)

			var orderDTO dto.CreateOrderOutput
			err := json.Unmarshal(d.Body, &orderDTO)
			if err != nil {
				log.Printf("Erro ao fazer parse do JSON: %v", err)
				continue
			}

			// 2. Simular busca de motorista (Regra de Negócio futura)
			fmt.Printf("Seeking driver for order %s...\n", orderDTO.ID)
			time.Sleep(2 * time.Second) // Simula latência (cálculo de rota)
			fmt.Println("Driver found and notified!")

			// 3. Acknowledge (Avisar ao RabbitMQ que pode apagar a mensagem)
			d.Ack(false)
		}
	}()

	fmt.Println(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever

	return nil
}
