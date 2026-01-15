package main

import (
	"database/sql"
	"fmt"
	"github.com/DioGolang/GoFleet/configs"
	"github.com/DioGolang/GoFleet/internal/application/usecase"
	"github.com/DioGolang/GoFleet/internal/domain/event"
	"github.com/DioGolang/GoFleet/internal/infra/database"
	infraEvent "github.com/DioGolang/GoFleet/internal/infra/event"
	"github.com/DioGolang/GoFleet/internal/infra/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"net/http"
)

func main() {
	config, err := configs.LoadConfig(".")
	if err != nil {
		panic(err)
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)

	db, err := sql.Open(config.DBDriver, dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		panic(err)
	}

	//RabbiMQ
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		panic(err)
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(
		"orders.created", // name
		true,             // durable
		false,            // delete when unused
		false,            // exclusive
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		panic(err)
	}

	err = ch.QueueBind(
		"orders.created",
		"orders.created",
		"amq.direct",
		false,
		nil,
	)
	if err != nil {
		panic(err)
	}

	orderRepository := database.NewOrderRepository(db)
	//events
	orderCreated := event.NewOrderCreated()
	eventDispatcher := infraEvent.NewDispatcher(ch)

	createOrderUseCase := usecase.NewCreateOrderUseCase(orderRepository, orderCreated, eventDispatcher)
	orderHandler := web.NewOrderHandler(createOrderUseCase)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/api/v1/orders", orderHandler.Create)

	fmt.Println("Server running on port", config.WebServerPort)

	log.Fatal(http.ListenAndServe(":"+config.WebServerPort, r))

}
