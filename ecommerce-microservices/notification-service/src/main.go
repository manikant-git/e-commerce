package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// ============================================================
// Models
// ============================================================

type NotificationType string

const (
	TypeEmail NotificationType = "EMAIL"
	TypeSMS   NotificationType = "SMS"
	TypePush  NotificationType = "PUSH"
)

type NotificationStatus string

const (
	StatusPending   NotificationStatus = "PENDING"
	StatusSent      NotificationStatus = "SENT"
	StatusFailed    NotificationStatus = "FAILED"
	StatusDelivered NotificationStatus = "DELIVERED"
)

type Notification struct {
	ID          string             `json:"id"`
	UserID      string             `json:"userId"`
	Type        NotificationType   `json:"type"`
	Subject     string             `json:"subject"`
	Message     string             `json:"message"`
	RecipientTo string             `json:"recipientTo"`
	Status      NotificationStatus `json:"status"`
	CreatedAt   time.Time          `json:"createdAt"`
	SentAt      *time.Time         `json:"sentAt,omitempty"`
}

type CreateNotificationRequest struct {
	UserID      string           `json:"userId"`
	Type        NotificationType `json:"type"`
	Subject     string           `json:"subject"`
	Message     string           `json:"message"`
	RecipientTo string           `json:"recipientTo"`
}

// ============================================================
// In-memory store (replace with DB in production)
// ============================================================

type Store struct {
	mu            sync.RWMutex
	notifications map[string]*Notification
	counter       int
}

var store = &Store{
	notifications: make(map[string]*Notification),
}

func (s *Store) Create(req CreateNotificationRequest) *Notification {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter++
	n := &Notification{
		ID:          fmt.Sprintf("notif-%05d", s.counter),
		UserID:      req.UserID,
		Type:        req.Type,
		Subject:     req.Subject,
		Message:     req.Message,
		RecipientTo: req.RecipientTo,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
	}
	s.notifications[n.ID] = n
	return n
}

func (s *Store) GetAll() []*Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Notification, 0, len(s.notifications))
	for _, n := range s.notifications {
		list = append(list, n)
	}
	return list
}

func (s *Store) GetByID(id string) (*Notification, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.notifications[id]
	return n, ok
}

func (s *Store) GetByUserID(userID string) []*Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*Notification
	for _, n := range s.notifications {
		if n.UserID == userID {
			list = append(list, n)
		}
	}
	return list
}

func (s *Store) UpdateStatus(id string, status NotificationStatus) (*Notification, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.notifications[id]
	if !ok {
		return nil, false
	}
	n.Status = status
	if status == StatusSent || status == StatusDelivered {
		now := time.Now()
		n.SentAt = &now
	}
	return n, true
}

// ============================================================
// Handlers
// ============================================================

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "UP", "service": "notification-service"})
}

func createNotificationHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}
	if req.UserID == "" || req.Message == "" || req.RecipientTo == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "userId, message and recipientTo are required"})
		return
	}
	if req.Type == "" {
		req.Type = TypeEmail
	}
	n := store.Create(req)

	// Simulate sending (async in production you'd use a queue)
	go func(notif *Notification) {
		time.Sleep(500 * time.Millisecond)
		log.Printf("[SEND] %s notification to %s: %s", notif.Type, notif.RecipientTo, notif.Subject)
		store.UpdateStatus(notif.ID, StatusSent)
	}(n)

	writeJSON(w, http.StatusCreated, n)
}

func getAllNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, store.GetAll())
}

func getNotificationByIDHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	n, ok := store.GetByID(vars["id"])
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Notification not found"})
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func getUserNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	notifications := store.GetByUserID(vars["userId"])
	if notifications == nil {
		notifications = []*Notification{}
	}
	writeJSON(w, http.StatusOK, notifications)
}

func updateNotificationStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var body struct {
		Status NotificationStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}
	n, ok := store.UpdateStatus(vars["id"], body.Status)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Notification not found"})
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// ============================================================
// Main
// ============================================================

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}

	r := mux.NewRouter()
	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/api/notifications", createNotificationHandler).Methods("POST")
	r.HandleFunc("/api/notifications", getAllNotificationsHandler).Methods("GET")
	r.HandleFunc("/api/notifications/{id}", getNotificationByIDHandler).Methods("GET")
	r.HandleFunc("/api/notifications/{id}/status", updateNotificationStatusHandler).Methods("PATCH")
	r.HandleFunc("/api/notifications/user/{userId}", getUserNotificationsHandler).Methods("GET")

	log.Printf("Notification Service listening on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
