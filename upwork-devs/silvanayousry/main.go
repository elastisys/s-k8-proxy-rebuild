package main

import (
	"context"
	"fmt"
	"log"
	"time"

	//"github.com/minio-go/pkg/credentials"

	"github.com/minio/minio-go"
	"github.com/minio/minio-go/pkg/credentials"
	"github.com/streadway/amqp"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func main() {

	ctx := context.Background()
	endpoint := "play.min.io"
	accessKeyID := "<Access Key>"
	secretAccessKey := "<Secret Key>"
	useSSL := true

	// Initialize minio client object.
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalln(err)
	}

	// Make a new bucket
	bucketName := "buckettt"
	location := "us-east-1"

	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: location})
	if err != nil {

		exists, errBucketExists := minioClient.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			log.Printf("We already own %s\n", bucketName)
		} else {
			log.Fatalln(err)
		}
	} else {
		log.Printf("Successfully created %s\n", bucketName)
	}

	// Upload a  file
	objectName := "example.pdf"
	filePath := "outgoing/example.pdf"
	contentType := "application/pdf"

	n, err := minioClient.FPutObject(ctx, bucketName, objectName, filePath, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Successfully uploaded %s of size %s \n", objectName, n)
	//time.Sleep(2 * time.Second)
	// Generates a url
	expiry := time.Second * 48 * 60 * 60
	presignedURL, err := minioClient.PresignedPutObject(context.Background(), "buckettt", "example.pdf", expiry)
	if err != nil {
		fmt.Println(err)
		return
	}
	x := presignedURL.String()
	time.Sleep(3 * time.Second)
	//rabbitmq
	fmt.Println("sending URL to rabbitmq")
	conn, err := amqp.Dial("amqp://<Username>:<Password>@localhost:5672/%2F")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"hello", // name
		false,   // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	failOnError(err, "Failed to declare a queue")

	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(x),
		})
	log.Printf(" [x] Sent %s", x)
	time.Sleep(3 * time.Second)
	fmt.Println(" downloading the file in  local directory incoming")
	err = minioClient.FGetObject(context.Background(), "buckettt", "example.pdf", "incoming/example.pdf", minio.GetObjectOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	failOnError(err, "Failed to publish a message")
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever

}
