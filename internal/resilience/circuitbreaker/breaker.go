package circuitbreaker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

var (
	// ErrOpen означает, что Circuit Breaker открыт
	// и временно отклоняет вызовы.
	ErrOpen = errors.New("circuit breaker is open")

	// ErrHalfOpenBusy означает, что проверочный вызов
	// в состоянии half-open уже выполняется.
	ErrHalfOpenBusy = errors.New(
		"circuit breaker half-open probe is already running",
	)

	// ErrNilOperation означает, что в Circuit Breaker
	// не передана операция.
	ErrNilOperation = errors.New(
		"circuit breaker operation is nil",
	)
)

// State описывает состояние Circuit Breaker.
type State uint8

const (
	// StateClosed пропускает вызовы к зависимости.
	StateClosed State = iota

	// StateOpen временно блокирует вызовы к зависимости.
	StateOpen

	// StateHalfOpen пропускает один проверочный вызов
	// после истечения open timeout.
	StateHalfOpen
)

// String возвращает строковое представление состояния.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"

	case StateOpen:
		return "open"

	case StateHalfOpen:
		return "half-open"

	default:
		return "unknown"
	}
}

// Config хранит настройки Circuit Breaker.
type Config struct {
	Enabled          bool
	FailureThreshold uint32
	OpenTimeout      time.Duration
}

// Breaker защищает вызовы одной внешней зависимости.
type Breaker struct {
	name             string
	enabled          bool
	failureThreshold uint32
	openTimeout      time.Duration
	log              *slog.Logger
	now              func() time.Time
	isFailure        func(error) bool

	mu                  sync.Mutex
	state               State
	consecutiveFailures uint32
	openedAt            time.Time
	halfOpenInFlight    bool
	generation          uint64
}

// requestTicket связывает завершение операции с состоянием,
// в котором операция была разрешена.
type requestTicket struct {
	state      State
	generation uint64
}

// stateChange описывает изменение состояния Circuit Breaker.
type stateChange struct {
	from State
	to   State
}

// New создаёт Circuit Breaker.
func New(
	name string,
	cfg Config,
	log *slog.Logger,
) *Breaker {
	return newBreaker(name, cfg, log, time.Now)
}

// newBreaker создаёт Circuit Breaker с заданными часами.
// Отдельный конструктор используется в тестах.
func newBreaker(
	name string,
	cfg Config,
	log *slog.Logger,
	now func() time.Time,
) *Breaker {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "external_dependency"
	}

	if cfg.FailureThreshold == 0 {
		cfg.FailureThreshold = 1
	}

	if cfg.OpenTimeout <= 0 {
		cfg.OpenTimeout = 30 * time.Second
	}

	if log == nil {
		log = slog.New(
			slog.NewTextHandler(io.Discard, nil),
		)
	}

	if now == nil {
		now = time.Now
	}

	return &Breaker{
		name:             name,
		enabled:          cfg.Enabled,
		failureThreshold: cfg.FailureThreshold,
		openTimeout:      cfg.OpenTimeout,
		log:              log,
		now:              now,
		isFailure:        defaultFailurePredicate,
		state:            StateClosed,
	}
}

// Execute выполняет операцию через Circuit Breaker.
func (b *Breaker) Execute(
	ctx context.Context,
	operation func() error,
) error {
	if operation == nil {
		return ErrNilOperation
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if b == nil || !b.enabled {
		return operation()
	}

	ticket, err := b.beforeRequest()
	if err != nil {
		return err
	}

	err = operation()
	b.afterRequest(ticket, err)

	return err
}

// ExecuteValue выполняет через Circuit Breaker операцию,
// которая возвращает значение и ошибку.
func ExecuteValue[T any](
	ctx context.Context,
	breaker *Breaker,
	operation func() (T, error),
) (T, error) {
	var zero T

	if operation == nil {
		return zero, ErrNilOperation
	}

	var result T

	err := breaker.Execute(ctx, func() error {
		var operationErr error

		result, operationErr = operation()

		return operationErr
	})
	if err != nil {
		return zero, err
	}

	return result, nil
}

// State возвращает текущее состояние Circuit Breaker.
func (b *Breaker) State() State {
	if b == nil || !b.enabled {
		return StateClosed
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	return b.state
}

// IsUnavailable проверяет ошибку временной блокировки вызова.
func IsUnavailable(err error) bool {
	return errors.Is(err, ErrOpen) ||
		errors.Is(err, ErrHalfOpenBusy)
}

func (b *Breaker) beforeRequest() (
	requestTicket,
	error,
) {
	b.mu.Lock()

	var change *stateChange

	switch b.state {
	case StateOpen:
		if b.now().Sub(b.openedAt) < b.openTimeout {
			b.mu.Unlock()

			return requestTicket{}, ErrOpen
		}

		change = b.transitionLocked(StateHalfOpen)
		b.halfOpenInFlight = true

	case StateHalfOpen:
		if b.halfOpenInFlight {
			b.mu.Unlock()

			return requestTicket{}, ErrHalfOpenBusy
		}

		b.halfOpenInFlight = true
	}

	ticket := requestTicket{
		state:      b.state,
		generation: b.generation,
	}

	b.mu.Unlock()

	b.logStateChange(change)

	return ticket, nil
}

func (b *Breaker) afterRequest(
	ticket requestTicket,
	err error,
) {
	if err == nil {
		b.recordSuccess(ticket)
		return
	}

	if !b.isFailure(err) {
		b.releaseHalfOpenProbe(ticket)
		return
	}

	b.recordFailure(ticket)
}

func (b *Breaker) recordSuccess(
	ticket requestTicket,
) {
	b.mu.Lock()

	if ticket.generation != b.generation {
		b.mu.Unlock()
		return
	}

	var change *stateChange

	switch ticket.state {
	case StateClosed:
		if b.state == StateClosed {
			b.consecutiveFailures = 0
		}

	case StateHalfOpen:
		if b.state == StateHalfOpen {
			b.consecutiveFailures = 0
			b.halfOpenInFlight = false
			change = b.transitionLocked(StateClosed)
		}
	}

	b.mu.Unlock()

	b.logStateChange(change)
}

func (b *Breaker) recordFailure(
	ticket requestTicket,
) {
	b.mu.Lock()

	if ticket.generation != b.generation {
		b.mu.Unlock()
		return
	}

	var change *stateChange

	switch ticket.state {
	case StateClosed:
		if b.state == StateClosed {
			b.consecutiveFailures++

			if b.consecutiveFailures >=
				b.failureThreshold {
				b.openedAt = b.now()
				change = b.transitionLocked(
					StateOpen,
				)
			}
		}

	case StateHalfOpen:
		if b.state == StateHalfOpen {
			b.openedAt = b.now()
			b.halfOpenInFlight = false
			change = b.transitionLocked(StateOpen)
		}
	}

	b.mu.Unlock()

	b.logStateChange(change)
}

func (b *Breaker) releaseHalfOpenProbe(
	ticket requestTicket,
) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ticket.generation == b.generation &&
		ticket.state == StateHalfOpen &&
		b.state == StateHalfOpen {
		b.halfOpenInFlight = false
	}
}

func (b *Breaker) transitionLocked(
	to State,
) *stateChange {
	if b.state == to {
		return nil
	}

	change := &stateChange{
		from: b.state,
		to:   to,
	}

	b.state = to
	b.generation++

	return change
}

func (b *Breaker) logStateChange(
	change *stateChange,
) {
	if change == nil {
		return
	}

	b.log.Info(
		"circuit breaker state changed",
		slog.String("circuit_breaker", b.name),
		slog.String("from", change.from.String()),
		slog.String("to", change.to.String()),
	)
}

func defaultFailurePredicate(err error) bool {
	return err != nil &&
		!errors.Is(err, context.Canceled)
}
