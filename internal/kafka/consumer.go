package kafka

import (
	"log"
	"os"

	"github.com/Shopify/sarama"
)

var Consumer sarama.Consumer

func InitConsumer() error {
	broker := os.Getenv("KAFKA_BROKER")
	if broker == "" {
		broker = "localhost:9092"
	}

	var err error
	Consumer, err = sarama.NewConsumer([]string{broker}, nil)
	if err != nil {
		return err
	}

	log.Println("Consumidor Kafka inicializado com sucesso")
	return nil
}

func ConsumeMessages(topic string, handler func([]byte) bool) error {
	partitionConsumer, err := Consumer.ConsumePartition(topic, 0, sarama.OffsetOldest)
	if err != nil {
		return err
	}
	defer partitionConsumer.Close()

	messageCount := 0
	for message := range partitionConsumer.Messages() {
		messageCount++

		// Parar se processou muitas mensagens (proteção)
		if messageCount > 10000 {
			log.Printf("Limite de segurança atingido (%d mensagens), parando", messageCount)
			break
		}

		if handler(message.Value) {
			log.Printf("Handler retornou true, parando consumo após %d mensagens", messageCount)
			break
		}
	}

	return nil
}

func CloseConsumer() {
	if Consumer != nil {
		Consumer.Close()
	}
}
