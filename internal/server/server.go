package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	_ "github.com/shuliakovsky/email-checker/docs"
	"github.com/shuliakovsky/email-checker/internal/checker"
	"github.com/shuliakovsky/email-checker/internal/lock"
	"github.com/shuliakovsky/email-checker/internal/metrics"
	"github.com/shuliakovsky/email-checker/internal/storage"
	"github.com/shuliakovsky/email-checker/pkg/types"
	httpSwagger "github.com/swaggo/http-swagger"
)

// Creates a new Server instance with specified configuration
func NewServer(port string, store storage.Storage, redisClient redis.UniversalClient, maxWorkers int, clusterMode bool) *Server {
	return &Server{
		storage:     store,
		redisClient: redisClient,
		port:        port,
		maxWorkers:  maxWorkers,
		clusterMode: clusterMode,
	}
}

// Processes tasks in local mode using in-memory queue
func (s *Server) localWorker() {
	for {
		task, err := s.storage.DequeueTask()
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		s.processTask(task)
	}
}

// Starts the HTTP server and task processing infrastructure
func (s *Server) Start() error {
	if s.clusterMode {
		s.startClusterTaskProcessor()
		s.startStalledTasksRecovery()
	} else {
		s.startLocalTaskProcessor()
	}

	router := http.NewServeMux()
	router.HandleFunc("/cache/flush", s.handleFlushCache)
	router.Handle("/metrics", promhttp.Handler())
	router.HandleFunc("/cache/status", s.handleCacheStatus)
	router.HandleFunc("/tasks", s.handleTasks)
	router.HandleFunc("/tasks/", s.handleTaskStatus)
	router.HandleFunc("/tasks-results/", s.handleTaskResults)
	router.HandleFunc("/tasks-with-webhook", s.handleTasksWithWebhook)
	router.HandleFunc("/swagger/", httpSwagger.WrapHandler)
	loggedRouter := loggingMiddleware(router)
	return http.ListenAndServe(":"+s.port, loggedRouter)
}

// Lua script for atomic task dequeue with lock acquisition
const dequeueScript = `
local task_data = redis.call('RPOP', KEYS[1])
if not task_data then return nil end
local task = cjson.decode(task_data)
local lock_key = 'lock:task:' .. task.id
if redis.call('SET', lock_key, ARGV[1], 'NX', 'EX', ARGV[2]) then
	return task_data
else
	redis.call('LPUSH', KEYS[1], task_data)
	return nil
end`

// Starts cluster-aware task processing workers
func (s *Server) startClusterTaskProcessor() {
	for i := 0; i < s.maxWorkers; i++ {
		go func() {
			for {
				task, err := s.dequeueTaskWithLock()
				if err != nil {
					time.Sleep(1 * time.Second)
					continue
				}
				s.processClusterTask(task)
			}
		}()
	}
}

// Atomically dequeues task with Redis lock acquisition
func (s *Server) dequeueTaskWithLock() (*types.Task, error) {
	result, err := s.redisClient.Eval(
		context.Background(),
		dequeueScript,
		[]string{storage.TaskQueueKey},
		fmt.Sprintf("worker:%d", time.Now().UnixNano()),
		300,
	).Result()

	if err != nil || result == nil {
		return nil, fmt.Errorf("no tasks available")
	}

	var task types.Task
	if err := json.Unmarshal([]byte(result.(string)), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// Periodically recovers stalled tasks with expired locks
func (s *Server) startStalledTasksRecovery() {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			script := `
				local locks = redis.call('KEYS', 'lock:task:*')
				for _, lock_key in ipairs(locks) do
					local ttl = redis.call('TTL', lock_key)
					if ttl == -1 or ttl < 60 then
						local task_id = string.sub(lock_key, 11)
						redis.call('LPUSH', KEYS[1], task_id)
						redis.call('DEL', lock_key)
					end
				end
			`
			s.redisClient.Eval(context.Background(), script, []string{storage.TaskQueueKey})
		}
	}()
}

// Processes task in cluster mode with distributed locking
func (s *Server) processClusterTask(task *types.Task) {
	lockKey := fmt.Sprintf("lock:task:%s", task.ID)
	lock := lock.NewLock(s.redisClient, lockKey, 5*time.Minute, s.clusterMode)

	if !lock.Acquire(context.Background()) {
		return
	}

	refreshCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lock.StartRefresh(refreshCtx)
	defer lock.Release(context.Background())

	task.Status = "processing"
	s.storage.UpdateTask(context.Background(), task)

	cfg := checker.Config{
		MaxWorkers:     s.maxWorkers,
		CacheProvider:  s.storage.GetCacheProvider(),
		DomainCacheTTL: 24 * time.Hour,
		ExistTTL:       720 * time.Hour,
		NotExistTTL:    24 * time.Hour,
	}

	results := checker.ProcessEmailsWithConfig(task.Emails, cfg)
	task.Status = "completed"
	task.Results = results

	s.storage.UpdateTask(context.Background(), task)
}

// Generates unique task ID using nanosecond timestamp
func (s *Server) generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Initializes local task processing workers
func (s *Server) startLocalTaskProcessor() {
	for i := 0; i < s.maxWorkers; i++ {
		go s.localWorker()
	}
}

// Handles task creation requests
func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var request struct {
			Emails []string `json:"emails"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		// limit emails length with 10 000
		if len(request.Emails) > 10000 {
			http.Error(w, "Too many emails (max 10000)", http.StatusBadRequest)
			return
		}
		// base check for email length
		for _, email := range request.Emails {
			if len(email) > 254 {
				http.Error(w, "Email too long", http.StatusBadRequest)
				return
			}
		}

		taskID := s.generateID()
		task := &types.Task{
			ID:        taskID,
			Status:    "pending",
			Emails:    request.Emails,
			CreatedAt: time.Now(),
		}

		if err := s.storage.SaveTask(r.Context(), task); err != nil {
			http.Error(w, "Failed to save task", http.StatusInternalServerError)
			return
		}

		go s.processTask(task)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"task_id": taskID})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// Provides task status information
func (s *Server) handleTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Path[len("/tasks/"):]

	task, err := s.storage.GetTask(r.Context(), taskID)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	var totalPages int
	if task.Status == "completed" {
		totalPages = (len(task.Results) + 99) / 100
	}

	response := TaskStatusResponse{
		Status:       task.Status,
		TotalResults: len(task.Results),
		CreatedAt:    task.CreatedAt,
		TotalPages:   totalPages,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Serves paginated task results
func (s *Server) handleTaskResults(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Path[len("/tasks-results/"):]
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	if perPage <= 0 {
		perPage = 100
	}
	if page <= 0 {
		page = 1
	}

	task, err := s.storage.GetTask(r.Context(), taskID)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	start := (page - 1) * perPage
	if start < 0 || start >= len(task.Results) {
		start = 0
	}
	end := start + perPage
	if end > len(task.Results) {
		end = len(task.Results)
	}

	response := struct {
		Data  []types.EmailReport `json:"data"`
		Page  int                 `json:"page"`
		Total int                 `json:"total"`
	}{
		Data:  task.Results[start:end],
		Page:  page,
		Total: len(task.Results),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Executes email validation task and updates state
func (s *Server) processTask(task *types.Task) {
	ctx := context.Background()
	task.Status = "processing"
	_ = s.storage.UpdateTask(ctx, task)

	cfg := checker.Config{
		MaxWorkers:     s.maxWorkers,
		CacheProvider:  s.storage.GetCacheProvider(),
		DomainCacheTTL: 24 * time.Hour,
		ExistTTL:       30 * 24 * time.Hour,
		NotExistTTL:    24 * time.Hour,
	}

	results := checker.ProcessEmailsWithConfig(task.Emails, cfg)
	task.Status = "completed"
	task.Results = results
	_ = s.storage.UpdateTask(ctx, task)
	if task.Webhook != nil {
		s.triggerWebhook(task)
	}
}

// Handles cache flush operations
func (s *Server) handleFlushCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.storage.GetCacheProvider().Flush()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Cache successfully flushed"))
}

// Provides cache system statistics
func (s *Server) handleCacheStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := s.storage.GetCacheProvider().GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// Adds request logging to HTTP handlers
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := newLoggingResponseWriter(w)
		next.ServeHTTP(lrw, r)

		statusCode := strconv.Itoa(lrw.statusCode)
		metrics.HttpRequests.WithLabelValues(
			r.Method,
			r.URL.Path,
			statusCode,
		).Inc()
	})
}
