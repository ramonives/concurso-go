package kafka

import (
	"encoding/json"
	"log"
	"os"

	"github.com/Shopify/sarama"
)

var Producer sarama.SyncProducer

func InitProducer() error {
	broker := os.Getenv("KAFKA_BROKER")
	if broker == "" {
		broker = "localhost:9092"
	}

	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5

	var err error
	Producer, err = sarama.NewSyncProducer([]string{broker}, config)
	if err != nil {
		return err
	}

	log.Println("Produtor Kafka inicializado com sucesso")
	return nil
}

func SendMessage(topic string, message interface{}) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(jsonData),
	}

	_, _, err = Producer.SendMessage(msg)
	return err
}

func CloseProducer() {
	if Producer != nil {
		Producer.Close()
	}
}
