---
description: Go language expertise for writing idiomatic, production-quality Go code. Use for Go development, concurrency patterns, error handling, testing, and module management. Triggers: go, golang, goroutine, channel, interface, struct, pointer, slice, map, defer, context, error, gin, echo, fiber, cobra, viper, gorm, sqlx, go mod, go test, effective go, errgroup, sync, mutex, waitgroup.
---

# Go Language Expertise

## Overview

Skill guide idiomatic, efficient, production Go. Cover concurrency, error handling, testing, modules per Effective Go.

## Key Concepts

### Error Handling

```go
import (
    "errors"
    "fmt"
)

// Define sentinel errors
var (
    ErrNotFound     = errors.New("resource not found")
    ErrUnauthorized = errors.New("unauthorized access")
)

// Custom error types with context
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// Error wrapping for context
func fetchUser(id string) (*User, error) {
    user, err := db.GetUser(id)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, fmt.Errorf("user %s: %w", id, ErrNotFound)
        }
        return nil, fmt.Errorf("fetching user %s: %w", id, err)
    }
    return user, nil
}

// Error checking with Is and As
func handleError(err error) {
    if errors.Is(err, ErrNotFound) {
        // Handle not found
    }

    var validErr *ValidationError
    if errors.As(err, &validErr) {
        // Handle validation error with access to Field and Message
    }
}
```

### Concurrency Patterns

```go
// Worker pool pattern
func workerPool(jobs <-chan Job, results chan<- Result, numWorkers int) {
    var wg sync.WaitGroup
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for job := range jobs {
                results <- process(job)
            }
        }()
    }
    wg.Wait()
    close(results)
}

// Context for cancellation and timeouts
func fetchWithTimeout(ctx context.Context, url string) ([]byte, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    return io.ReadAll(resp.Body)
}

// Select for multiple channels
func multiplex(ctx context.Context, ch1, ch2 <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for {
            select {
            case v, ok := <-ch1:
                if !ok {
                    ch1 = nil
                    continue
                }
                out <- v
            case v, ok := <-ch2:
                if !ok {
                    ch2 = nil
                    continue
                }
                out <- v
            case <-ctx.Done():
                return
            }
            if ch1 == nil && ch2 == nil {
                return
            }
        }
    }()
    return out
}

// Mutex for shared state
type SafeCounter struct {
    mu    sync.RWMutex
    count map[string]int
}

func (c *SafeCounter) Inc(key string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count[key]++
}

func (c *SafeCounter) Get(key string) int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.count[key]
}
```

### Interfaces and Embedding

```go
// Small, focused interfaces
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

type ReadWriter interface {
    Reader
    Writer
}

// Accept interfaces, return structs
type UserRepository interface {
    GetByID(ctx context.Context, id string) (*User, error)
    Create(ctx context.Context, user *User) error
}

type userService struct {
    repo   UserRepository
    cache  Cache
    logger *slog.Logger
}

func NewUserService(repo UserRepository, cache Cache, logger *slog.Logger) *userService {
    return &userService{
        repo:   repo,
        cache:  cache,
        logger: logger,
    }
}

// Embedding for composition
type Base struct {
    ID        string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type User struct {
    Base
    Email string
    Name  string
}
```

### Functional Options Pattern

```go
type Server struct {
    host    string
    port    int
    timeout time.Duration
    logger  *slog.Logger
}

type Option func(*Server)

func WithHost(host string) Option {
    return func(s *Server) {
        s.host = host
    }
}

func WithPort(port int) Option {
    return func(s *Server) {
        s.port = port
    }
}

func WithTimeout(d time.Duration) Option {
    return func(s *Server) {
        s.timeout = d
    }
}

func WithLogger(logger *slog.Logger) Option {
    return func(s *Server) {
        s.logger = logger
    }
}

func NewServer(opts ...Option) *Server {
    s := &Server{
        host:    "localhost",
        port:    8080,
        timeout: 30 * time.Second,
        logger:  slog.Default(),
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage
server := NewServer(
    WithHost("0.0.0.0"),
    WithPort(9000),
    WithTimeout(60*time.Second),
)
```

### CLI Applications with Cobra

```go
// cmd/root.go
package cmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var (
    cfgFile string
    verbose bool
)

var rootCmd = &cobra.Command{
    Use:   "myapp",
    Short: "A brief description of your application",
    Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application.`,
}

func Execute() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

func init() {
    cobra.OnInitialize(initConfig)

    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.myapp.yaml)")
    rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

    viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

func initConfig() {
    if cfgFile != "" {
        viper.SetConfigFile(cfgFile)
    } else {
        home, err := os.UserHomeDir()
        cobra.CheckErr(err)
        viper.AddConfigPath(home)
        viper.SetConfigType("yaml")
        viper.SetConfigName(".myapp")
    }

    viper.AutomaticEnv()

    if err := viper.ReadInConfig(); err == nil {
        fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
    }
}

// cmd/serve.go
var serveCmd = &cobra.Command{
    Use:   "serve",
    Short: "Start the HTTP server",
    RunE: func(cmd *cobra.Command, args []string) error {
        port := viper.GetInt("port")
        return startServer(cmd.Context(), port)
    },
}

func init() {
    rootCmd.AddCommand(serveCmd)
    serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
    viper.BindPFlag("port", serveCmd.Flags().Lookup("port"))
}
```

### HTTP Servers

```go
// Standard library HTTP server
package server

import (
    "context"
    "errors"
    "log/slog"
    "net/http"
    "time"
)

type Server struct {
    httpServer *http.Server
    logger     *slog.Logger
}

func New(addr string, handler http.Handler, logger *slog.Logger) *Server {
    return &Server{
        httpServer: &http.Server{
            Addr:         addr,
            Handler:      handler,
            ReadTimeout:  15 * time.Second,
            WriteTimeout: 15 * time.Second,
            IdleTimeout:  60 * time.Second,
        },
        logger: logger,
    }
}

func (s *Server) Start() error {
    s.logger.Info("starting server", slog.String("addr", s.httpServer.Addr))
    if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        return err
    }
    return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
    s.logger.Info("shutting down server")
    return s.httpServer.Shutdown(ctx)
}

// Middleware patterns
func loggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            next.ServeHTTP(w, r)
            logger.Info("request",
                slog.String("method", r.Method),
                slog.String("path", r.URL.Path),
                slog.Duration("duration", time.Since(start)),
            )
        })
    }
}

func recoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if err := recover(); err != nil {
                    logger.Error("panic recovered",
                        slog.Any("error", err),
                        slog.String("path", r.URL.Path),
                    )
                    http.Error(w, "internal server error", http.StatusInternalServerError)
                }
            }()
            next.ServeHTTP(w, r)
        })
    }
}

// Router setup with Go 1.22+ patterns
func setupRoutes(mux *http.ServeMux, handler *Handler) {
    mux.HandleFunc("GET /api/users/{id}", handler.GetUser)
    mux.HandleFunc("POST /api/users", handler.CreateUser)
    mux.HandleFunc("PUT /api/users/{id}", handler.UpdateUser)
    mux.HandleFunc("DELETE /api/users/{id}", handler.DeleteUser)
    mux.HandleFunc("GET /health", handler.Health)
}

// Gin framework example
import "github.com/gin-gonic/gin"

func setupGinRouter(handler *Handler) *gin.Engine {
    r := gin.New()
    r.Use(gin.Recovery())
    r.Use(gin.Logger())

    api := r.Group("/api")
    {
        users := api.Group("/users")
        {
            users.GET("/:id", handler.GetUser)
            users.POST("/", handler.CreateUser)
            users.PUT("/:id", handler.UpdateUser)
            users.DELETE("/:id", handler.DeleteUser)
        }
    }

    r.GET("/health", handler.Health)
    return r
}
```

### Database Access Patterns

```go
// Using sqlx for cleaner database operations
import (
    "context"
    "database/sql"
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
)

type User struct {
    ID        string    `db:"id"`
    Email     string    `db:"email"`
    Name      string    `db:"name"`
    CreatedAt time.Time `db:"created_at"`
}

type UserRepository struct {
    db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
    return &UserRepository{db: db}
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*User, error) {
    var user User
    query := `SELECT id, email, name, created_at FROM users WHERE id = $1`

    if err := r.db.GetContext(ctx, &user, query, id); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, ErrNotFound
        }
        return nil, fmt.Errorf("getting user: %w", err)
    }
    return &user, nil
}

func (r *UserRepository) Create(ctx context.Context, user *User) error {
    query := `
        INSERT INTO users (id, email, name, created_at)
        VALUES (:id, :email, :name, :created_at)
    `

    _, err := r.db.NamedExecContext(ctx, query, user)
    if err != nil {
        return fmt.Errorf("creating user: %w", err)
    }
    return nil
}

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]*User, error) {
    var users []*User
    query := `
        SELECT id, email, name, created_at
        FROM users
        ORDER BY created_at DESC
        LIMIT $1 OFFSET $2
    `

    if err := r.db.SelectContext(ctx, &users, query, limit, offset); err != nil {
        return nil, fmt.Errorf("listing users: %w", err)
    }
    return users, nil
}

// Transaction helper
func (r *UserRepository) WithTx(ctx context.Context, fn func(*sqlx.Tx) error) error {
    tx, err := r.db.BeginTxx(ctx, nil)
    if err != nil {
        return fmt.Errorf("beginning transaction: %w", err)
    }

    if err := fn(tx); err != nil {
        if rbErr := tx.Rollback(); rbErr != nil {
            return fmt.Errorf("rolling back transaction: %v (original error: %w)", rbErr, err)
        }
        return err
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("committing transaction: %w", err)
    }
    return nil
}
```

## Best Practices

### Code Organization

```text
myproject/
├── cmd/
│   └── myapp/
│       └── main.go
├── internal/
│   ├── service/
│   ├── repository/
│   └── handler/
├── pkg/
│   └── utils/
├── go.mod
└── go.sum
```

### Module Management

```go
// go.mod
module github.com/user/myproject

go 1.22

require (
    github.com/lib/pq v1.10.9
    golang.org/x/sync v0.5.0
)
```

### Structured Logging with slog

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

logger.Info("request processed",
    slog.String("method", r.Method),
    slog.String("path", r.URL.Path),
    slog.Duration("latency", time.Since(start)),
)

// Add context to logger
logger = logger.With(slog.String("request_id", requestID))
```

## Common Patterns

### Table-Driven Tests

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive numbers", 2, 3, 5},
        {"negative numbers", -1, -2, -3},
        {"zero", 0, 0, 0},
        {"mixed", -1, 5, 4},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Add(tt.a, tt.b)
            if result != tt.expected {
                t.Errorf("Add(%d, %d) = %d; want %d", tt.a, tt.b, result, tt.expected)
            }
        })
    }
}

// Parallel subtests
func TestFetch(t *testing.T) {
    tests := []struct {
        name string
        url  string
    }{
        {"google", "https://google.com"},
        {"github", "https://github.com"},
    }

    for _, tt := range tests {
        tt := tt // capture range variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            // test implementation
        })
    }
}
```

### Benchmarks

```go
func BenchmarkFibonacci(b *testing.B) {
    for i := 0; i < b.N; i++ {
        Fibonacci(20)
    }
}

func BenchmarkFibonacciParallel(b *testing.B) {
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            Fibonacci(20)
        }
    })
}

// With sub-benchmarks
func BenchmarkSort(b *testing.B) {
    sizes := []int{100, 1000, 10000}
    for _, size := range sizes {
        b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
            data := generateData(size)
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                Sort(data)
            }
        })
    }
}
```

### HTTP Handlers

```go
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    id := r.PathValue("id") // Go 1.22+

    user, err := h.service.GetUser(ctx, id)
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            http.Error(w, "user not found", http.StatusNotFound)
            return
        }
        h.logger.Error("failed to get user", slog.String("error", err.Error()))
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(user); err != nil {
        h.logger.Error("failed to encode response", slog.String("error", err.Error()))
    }
}
```

## Anti-Patterns

### Avoid These Practices

```go
// BAD: Ignoring errors
result, _ := doSomething()

// GOOD: Always handle errors
result, err := doSomething()
if err != nil {
    return fmt.Errorf("doing something: %w", err)
}

// BAD: Goroutine leaks
func fetch(urls []string) []Result {
    results := make(chan Result)
    for _, url := range urls {
        go func(u string) {
            results <- fetchURL(u) // Blocks forever if nobody reads
        }(url)
    }
    return collectResults(results)
}

// GOOD: Use context and proper cleanup
func fetch(ctx context.Context, urls []string) ([]Result, error) {
    g, ctx := errgroup.WithContext(ctx)
    results := make([]Result, len(urls))

    for i, url := range urls {
        i, url := i, url
        g.Go(func() error {
            r, err := fetchURL(ctx, url)
            if err != nil {
                return err
            }
            results[i] = r
            return nil
        })
    }

    if err := g.Wait(); err != nil {
        return nil, err
    }
    return results, nil
}

// BAD: Returning interfaces
func NewService() ServiceInterface {
    return &service{}
}

// GOOD: Return concrete types
func NewService() *Service {
    return &Service{}
}

// BAD: Large interfaces
type Repository interface {
    GetUser(id string) (*User, error)
    CreateUser(user *User) error
    UpdateUser(user *User) error
    DeleteUser(id string) error
    ListUsers() ([]*User, error)
    GetOrder(id string) (*Order, error)
    // ... 20 more methods
}

// GOOD: Small, focused interfaces
type UserGetter interface {
    GetUser(ctx context.Context, id string) (*User, error)
}

// BAD: Naked returns in long functions
func process(data []byte) (result string, err error) {
    // 50 lines of code
    result = string(data)
    return // What's being returned?
}

// GOOD: Explicit returns
func process(data []byte) (string, error) {
    // processing logic
    return string(data), nil
}

// BAD: init() for complex initialization
func init() {
    db = connectToDatabase()
    cache = initCache()
}

// GOOD: Explicit initialization in main
func main() {
    db, err := connectToDatabase()
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    cache := initCache()
    // ...
}
```
