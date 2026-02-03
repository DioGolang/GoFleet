package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/DioGolang/GoFleet/internal/application/port/outbound"
	"github.com/DioGolang/GoFleet/internal/application/usecase/order"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/pb"
	"github.com/DioGolang/GoFleet/pkg/logger"
	carrier "github.com/DioGolang/GoFleet/pkg/otel"
	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	MaxRetries = 3
	DLXName    = "dlx_exchange"
	MainEx     = "orders_exchange"
)

type Consumer struct {
	Conn            *amqp.Connection
	GrpcClient      pb.FleetServiceClient
	OrderRepository outbound.OrderRepository
	DispatchUseCase order.DispatchUseCase
	RedisClient     *redis.Client
	Logger          logger.Logger
	WorkerCount     int
}

func NewConsumer(
	conn *amqp.Connection,
	grpcClient pb.FleetServiceClient,
	repo outbound.OrderRepository,
	dispatchUseCase order.DispatchUseCase,
	redisClient *redis.Client,
	l logger.Logger,
	workerCount int,
) *Consumer {
	if workerCount <= 0 {
		workerCount = 1
	}
	return &Consumer{
		Conn:            conn,
		GrpcClient:      grpcClient,
		OrderRepository: repo,
		DispatchUseCase: dispatchUseCase,
		RedisClient:     redisClient,
		Logger:          l,
		WorkerCount:     workerCount,
	}
}

func (c *Consumer) Start(ctx context.Context, queueName string, handler MessageHandler) error {
	ch, err := c.Conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	// O Prefetch Count DEVE ser >= WorkerCount.
	prefetchCount := c.WorkerCount * 2
	if err := ch.Qos(prefetchCount, 0, false); err != nil {
		return fmt.Errorf("failed to set qos: %w", err)
	}

	if err := c.setupTopology(ch, queueName); err != nil {
		return fmt.Errorf("error configuration topology: %w", err)
	}

	msgs, err := ch.Consume(
		queueName, "", false, false, false, false, nil,
	)
	if err != nil {
		return err
	}

	c.Logger.Info(ctx, "Starting Worker Pool",
		logger.String("queue", queueName),
		logger.Int("workers", c.WorkerCount),
		logger.Int("prefetch", prefetchCount),
	)

	var wg sync.WaitGroup

	for i := 0; i < c.WorkerCount; i++ {
		wg.Add(1)
		go c.startWorker(ctx, &wg, i, msgs, handler, queueName, ch)
	}

	<-ctx.Done()
	c.Logger.Info(ctx, "Shutdown signal received. Closing channel and waiting for workers...")

	// Ao fechar o Channel do AMQP (via defer ou explicitamente),
	// o canal 'msgs' será fechado, fazendo os loops dos workers terminarem.
	// Mas para garantir, aguardamos o WaitGroup.
	ch.Close()
	wg.Wait()

	c.Logger.Info(ctx, "All workers stopped. Consumer shutdown complete.")
	return nil
}

func (c *Consumer) startWorker(
	ctx context.Context,
	wg *sync.WaitGroup,
	workerID int,
	msgs <-chan amqp.Delivery,
	handler MessageHandler,
	queueName string,
	ch *amqp.Channel,
) {
	defer wg.Done()

	// Safety: Recuperação de Panic para não derrubar a aplicação inteira
	// se um worker encontrar um bug bizarro.
	defer func() {
		if r := recover(); r != nil {
			c.Logger.Error(ctx, "Worker panicked!",
				logger.Int("worker_id", workerID),
				logger.Any("panic", r),
			)
		}
	}()

	c.Logger.Debug(ctx, "Worker started", logger.Int("worker_id", workerID))

	for d := range msgs {
		c.handleMessage(ctx, d, handler, queueName, ch)
	}

	c.Logger.Debug(ctx, "Worker stopped", logger.Int("worker_id", workerID))
}

func (c *Consumer) handleMessage(ctx context.Context, d amqp.Delivery, handler MessageHandler, queueName string, ch *amqp.Channel) {
	amqpCarrier := carrier.AMQPHeadersCarrier(d.Headers)
	ctx = otel.GetTextMapPropagator().Extract(ctx, amqpCarrier)
	tracer := otel.GetTracerProvider().Tracer("worker-tracer")

	ctx, span := tracer.Start(ctx, "ProcessOrder", trace.WithAttributes(
		attribute.String("queue.name", queueName),
		attribute.String("messaging.message_id", d.MessageId),
	))
	defer span.End()

	err := handler(ctx, d.Body, d.Headers)

	// --- CENÁRIO: SUCESSO ---
	if err == nil {
		c.Logger.Info(ctx, "Message processed successfully", logger.String("msg_id", d.MessageId))
		d.Ack(false) // CRÍTICO: Não esqueça o Ack!
		return
	}

	// --- CENÁRIO: FALHA ---
	retryCount := c.getRetryCount(d)
	c.Logger.Warn(ctx, "Processing failed",
		logger.WithError(err),
		logger.Int("retry_count", int(retryCount)),
	)

	// A. Circuit Breaker Aberto -> Tenta Fallback
	if errors.Is(err, gobreaker.ErrOpenState) {
		c.Logger.Warn(ctx, "Circuit Breaker Open. Attempting Fallback...")
		if fbErr := c.executeFallback(ctx, d.Body); fbErr == nil {
			c.Logger.Info(ctx, "Fallback success. Discarding original message.")
			d.Ack(false) // Fallback tratou, vida que segue.
			return
		} else {
			c.Logger.Error(ctx, "Fallback failed too.", logger.WithError(fbErr))
			// Se fallback falhou, cai na lógica de retry abaixo
		}
	}

	// B. Excedeu Retries -> Manda para Parking (Cemitério)
	if retryCount >= MaxRetries {
		c.Logger.Error(ctx, "Max retries reached. Moving to Parking Queue.", logger.String("msg_id", d.MessageId))
		if pubErr := c.publishToParking(ch, queueName, d); pubErr != nil {
			c.Logger.Error(ctx, "CRITICAL: Failed to publish to parking!", logger.WithError(pubErr))
			// Se não consegue nem mandar pro parking, Nack com Requeue para tentar salvar dnv
			d.Nack(false, true)
			return
		}
		d.Ack(false) // Remove da fila principal pois já está na parking
		return
	}

	// C. Erro Transiente ou Retry Padrão -> Manda para DLX (Wait Queue)
	// Nack(false, false) rejeita a msg SEM requeue.
	// A configuração da topologia enviará para a DLX -> Wait Queue -> TTL -> Main Queue
	c.Logger.Info(ctx, "Sending message to DLQ (Wait Queue) for retry", logger.Int("next_retry", int(retryCount)+1))
	d.Nack(false, false)
}

func (c *Consumer) ProcessOrder(ctx context.Context, msg []byte, headers map[string]interface{}) error {
	// 1. Identidade do Evento (Idempotency Key)
	eventID := c.extractEventID(headers, msg)
	if eventID == "" {
		// Se não tiver ID, é perigoso processar. Mas como fallback, geramos um hash ou logamos erro.
		c.Logger.Warn(ctx, "Message without ID, skipping idempotency check")
	}

	// 2. Chave no Redis: "idem:orders:{event_id}"
	// TTL de 24h é suficiente para evitar deduplicação imediata.
	idempotencyKey := fmt.Sprintf("idem:orders:%s", eventID)

	if eventID != "" {
		// SETNX: "Set if Not Exists". Operação atômica.
		// Se retornar true: A chave não existia, setou e ganhou o lock.
		// Se retornar false: A chave já existe, alguém já processou.
		acquired, err := c.RedisClient.SetNX(ctx, idempotencyKey, "processing", 24*time.Hour).Result()
		if err != nil {
			// Falha no Redis (Infra). Retornamos erro para dar Nack e tentar de novo.
			return fmt.Errorf("redis failure: %w", err)
		}
		if !acquired {
			// DUPLICIDADE DETECTADA!
			c.Logger.Info(ctx, "Event already processed (Idempotency Hit). Skipping.",
				logger.String("event_id", eventID))
			return nil // Retorna nil para dar ACK e remover da fila.
		}
	}

	// --- PONTO CRÍTICO: Tratamento de Erro com Idempotência ---
	err := c.executeBusinessLogic(ctx, msg)

	if err != nil {
		c.Logger.Warn(ctx, "Processing failed, releasing idempotency key for retry",
			logger.String("event_id", eventID),
			logger.WithError(err))

		if eventID != "" {
			c.RedisClient.Del(ctx, idempotencyKey)
		}

		return err // Retorna o erro para o handler jogar na Wait Queue
	}
	return nil
}

func (c *Consumer) executeBusinessLogic(ctx context.Context, msg []byte) error {
	var orderDto order.CreateInput
	if err := json.Unmarshal(msg, &orderDto); err != nil {
		// Erro Fatal (JSON inválido). Não adianta retentar.
		// Retornamos nil ou um erro específico que o handler saiba descartar (Poison Message).
		// Aqui vou retornar erro encapsulado para o handler decidir (geralmente Nack False).
		return fmt.Errorf("invalid json: %w", err)
	}

	req := &pb.SearchDriverRequest{OrderId: orderDto.ID}
	res, err := c.GrpcClient.SearchDriver(ctx, req)
	if err != nil {
		return fmt.Errorf("grpc search driver failed: %w", err)
	}

	input := order.DispatchInput{OrderID: orderDto.ID, DriverID: res.DriverId}

	// AQUI MORA A CONSISTÊNCIA EVENTUAL
	// Se o DispatchUseCase buscar o pedido no banco e não achar (porque o evento chegou antes da escrita),
	// ele deve retornar um erro.
	if err := c.DispatchUseCase.Execute(ctx, input); err != nil {
		return err // Isso fará o Redis Key ser deletado e o msg ir pra Wait Queue
	}

	return nil
}

// Helper para extrair ID
func (c *Consumer) extractEventID(headers map[string]interface{}, msg []byte) string {
	if val, ok := headers["x-event-id"]; ok {
		switch v := val.(type) {
		case string:
			return v
		case []byte:
			return string(v)
		}
	}

	var payload struct {
		ID      string `json:"id"`
		EventID string `json:"event_id"`
	}

	if err := json.Unmarshal(msg, &payload); err == nil {
		if payload.ID != "" {
			return payload.ID
		}
		if payload.EventID != "" {
			return payload.EventID
		}
	}

	return ""
}

// Helper Resilience

// setupTopology Main Queue, DLX, Wait Queue e Parking Queue
func (c *Consumer) setupTopology(ch *amqp.Channel, queueName string) error {
	// 1. DLX (Onde caem os rejeitados/erros)
	if err := ch.ExchangeDeclare(DLXName, "direct", true, false, false, false, nil); err != nil {
		return err
	}

	// 2. Main Exchange
	if err := ch.ExchangeDeclare(MainEx, "direct", true, false, false, false, nil); err != nil {
		return err
	}

	// 3. Wait Queue (A Fila de Espera)
	waitQueue := queueName + ".wait"
	argsWait := amqp.Table{
		"x-dead-letter-exchange":    MainEx,    // Depois do TTL, volta pro Main Exchange
		"x-dead-letter-routing-key": queueName, // Com a routing key original
		"x-message-ttl":             10000,     // 10s de espera (Backoff fixo)
	}
	if _, err := ch.QueueDeclare(waitQueue, true, false, false, false, argsWait); err != nil {
		return err
	}
	// Bind da Wait Queue na DLX: Quando rejeitamos na Main, vai pra DLX, que joga na Wait
	if err := ch.QueueBind(waitQueue, queueName, DLXName, false, nil); err != nil {
		return err
	}

	// 4. Main Queue (A Fila de Trabalho)
	argsMain := amqp.Table{
		"x-dead-letter-exchange":    DLXName,   // Se der Nack(false, false), vai pra DLX
		"x-dead-letter-routing-key": queueName, // Mantém a routing key pra cair na Wait Queue correta
	}
	if _, err := ch.QueueDeclare(queueName, true, false, false, false, argsMain); err != nil {
		return err
	}
	if err := ch.QueueBind(queueName, queueName, MainEx, false, nil); err != nil {
		return err
	}

	// 5. Parking Queue (Fim da linha)
	parkingQueue := queueName + ".parking"
	if _, err := ch.QueueDeclare(parkingQueue, true, false, false, false, nil); err != nil {
		return err
	}

	return nil
}

func (c *Consumer) getRetryCount(msg amqp.Delivery) int64 {
	xDeath, ok := msg.Headers["x-death"].([]interface{})
	if !ok || len(xDeath) == 0 {
		return 0
	}
	for _, death := range xDeath {
		if table, ok := death.(amqp.Table); ok {
			// Verifica se o motivo foi 'rejected' (nosso Nack) ou 'expired' (TTL)
			// Geralmente contamos quantas vezes ele passou pela Wait Queue ou foi rejeitado.
			// Simplificação: Pegamos o 'count' geral.
			if count, ok := table["count"].(int64); ok {
				return count
			}
		}
	}
	return 0
}

func (c *Consumer) executeFallback(ctx context.Context, msg []byte) error {
	var dto order.CreateInput
	if err := json.Unmarshal(msg, &dto); err != nil {
		return fmt.Errorf("fallback unmarshal error: %w", err)
	}

	orderEntity, err := c.OrderRepository.FindByID(ctx, dto.ID)
	if err != nil {
		return fmt.Errorf("fallback find order error: %w", err)
	}

	if err := orderEntity.SendToManual(); err != nil {
		return fmt.Errorf("fallback domain transition error: %w", err)
	}

	err = c.OrderRepository.UpdateStatus(
		ctx,
		orderEntity.ID(),
		orderEntity.StatusName(), // "MANUAL_DISPATCH"
		orderEntity.DriverID(),
	)
	if err != nil {
		return fmt.Errorf("fallback save error: %w", err)
	}
	return nil
}

func (c *Consumer) publishToParking(ch *amqp.Channel, originalQueue string, msg amqp.Delivery) error {
	parkingQueue := originalQueue + ".parking"

	headers := msg.Headers
	if headers == nil {
		headers = make(amqp.Table)
	}
	headers["x-original-queue"] = originalQueue
	headers["x-fail-reason"] = "max-retries-exceeded"

	return ch.PublishWithContext(
		context.Background(),
		"", // Default Exchange
		parkingQueue,
		false,
		false,
		amqp.Publishing{
			Headers:      headers,
			ContentType:  msg.ContentType,
			Body:         msg.Body,
			DeliveryMode: amqp.Persistent,
		},
	)
}
