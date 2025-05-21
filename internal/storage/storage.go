package storage

import (
	"context"
	"encoding/json"
	"fmt"
	policymodel "github.com/1rp-pw/orchestrator/internal/policy"
	"github.com/bugfixes/go-bugfixes/logs"
	"github.com/google/uuid"
	ConfigBuilder "github.com/keloran/go-config"
	"github.com/segmentio/kafka-go"
	"io"
	"net/http"
	"time"
)

type System struct {
	Config  *ConfigBuilder.Config
	Context context.Context
}

func NewSystem(cfg *ConfigBuilder.Config) *System {
	return &System{
		Config:  cfg,
		Context: context.Background(),
	}
}

func (s *System) SetContext(ctx context.Context) *System {
	s.Context = ctx
	return s
}

func (s *System) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	s.SetContext(r.Context())
	uid, err := uuid.NewUUID()
	if err != nil {
		_ = logs.Errorf("failed to create policy id: %v", err)
		http.Error(w, "failed to create policy", http.StatusInternalServerError)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			_ = logs.Errorf("error closing body: %v", err)
		}
	}()

	var policy policymodel.Policy
	if err := json.Unmarshal(bodyBytes, &policy); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	policy.Version = "draft"
	policy.ID = uid.String()
	policy.CreatedAt = time.Now()

	kafkaHost, ok := s.Config.ProjectProperties["kafka_host"].(string)
	if !ok {
		_ = logs.Error("kafka_host config missing or invalid")
		http.Error(w, "failed to create policy", http.StatusInternalServerError)
		return
	}

	topic := fmt.Sprintf("policy-%s", uid.String())

	writer, err := kafka.DialLeader(r.Context(), "tcp", kafkaHost, topic, 0)
	if err != nil {
		_ = logs.Errorf("failed to connect to kafka: %v", err)
		http.Error(w, "failed to create policy", http.StatusInternalServerError)
	}

	data, err := json.Marshal(policy)
	if err != nil {
		_ = logs.Errorf("failed to marshal policy: %v", err)
		http.Error(w, "failed to create policy", http.StatusInternalServerError)
		return
	}

	if _, err = writer.WriteMessages(kafka.Message{
		Key:   []byte(policy.ID),
		Value: data,
	}); err != nil {
		_ = logs.Errorf("failed to write message: %v", err)
		http.Error(w, "failed to create policy", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(&policy); err != nil {
		_ = logs.Errorf("failed to encode response: %v", err)
		http.Error(w, "failed to create policy", http.StatusInternalServerError)
		return
	}
}

func (s *System) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	s.SetContext(r.Context())
	policyId := r.PathValue("policyId")

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			_ = logs.Errorf("error closing body: %v", err)
		}
	}()

	var policy policymodel.Policy
	if err := json.Unmarshal(bodyBytes, &policy); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	policy.ID = policyId
	policy.CreatedAt = time.Now()

	kafkaHost, ok := s.Config.ProjectProperties["kafka_host"].(string)
	if !ok {
		_ = logs.Error("kafka_host config missing or invalid")
		http.Error(w, "failed to update policy", http.StatusInternalServerError)
		return
	}

	topic := fmt.Sprintf("policy-%s", policyId)

	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  []string{kafkaHost},
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	})
	defer writer.Close()

	data, err := json.Marshal(policy)
	if err != nil {
		_ = logs.Errorf("failed to marshal policy: %v", err)
		http.Error(w, "failed to update policy", http.StatusInternalServerError)
		return
	}

	err = writer.WriteMessages(s.Context, kafka.Message{
		Key:   []byte(policy.ID),
		Value: data,
	})
	if err != nil {
		_ = logs.Errorf("failed to write message: %v", err)
		http.Error(w, "failed to update policy", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(&policy); err != nil {
		_ = logs.Errorf("failed to encode response: %v", err)
		http.Error(w, "failed to update policy", http.StatusInternalServerError)
		return
	}
}

func (s *System) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *System) GetPolicy(w http.ResponseWriter, r *http.Request) {
	s.Context = r.Context()

	policyId := r.PathValue("policyId")
	p, err := s.GetLatestPolicyFromStorage(policyId)
	if err != nil {
		//_ = logs.Errorf("Failed to get policy from storage: %v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if _, err := w.Write(p.([]byte)); err != nil {
		_ = logs.Errorf("Failed to write response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *System) GetPolicyVersions(w http.ResponseWriter, r *http.Request) {
	s.Context = r.Context()
	policyId := r.PathValue("policyId")
	p, err := s.GetPolicyFromStorage(policyId)
	if err != nil {
		//_ = logs.Errorf("Failed to get policy from storage: %v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(p); err != nil {
		_ = logs.Errorf("Failed to write response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *System) GetPolicyFromStorage(policyId string) ([]interface{}, error) {
	kafkaHost, ok := s.Config.ProjectProperties["kafka_host"].(string)
	if !ok {
		return nil, fmt.Errorf("kafka_host config missing or invalid")
	}

	topic := fmt.Sprintf("policy-%s", policyId)
	partition := 0

	conn, err := kafka.DialLeader(s.Context, "tcp", kafkaHost, topic, partition)
	if err != nil {
		return nil, fmt.Errorf("failed to dial leader: %w", err)
	}
	defer conn.Close()

	low, high, err := conn.ReadOffsets()
	if err != nil {
		return nil, fmt.Errorf("failed to read watermark offsets: %w", err)
	}

	messageCount := high - low
	if messageCount <= 0 {
		return nil, fmt.Errorf("no policy found")
	}

	var messages []interface{}

	// Read messages from low offset up to high-1
	for offset := low; offset < high; offset++ {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		msg, err := conn.ReadMessage(1e6) // max 1MB
		if err != nil {
			_ = logs.Errorf("failed to read message at offset %d: %v", offset, err)
			continue
		}

		var message interface{}
		if err := json.Unmarshal(msg.Value, &message); err != nil {
			_ = logs.Errorf("failed to unmarshal message at offset %d: %v", offset, err)
			continue
		}
		messages = append(messages, message)
	}

	return messages, nil
}

func (s *System) GetLatestPolicyFromStorage(pid string) (interface{}, error) {
	kafkaHost, ok := s.Config.ProjectProperties["kafka_host"].(string)
	if !ok {
		return nil, fmt.Errorf("kafka_host config missing or invalid")
	}

	topic := fmt.Sprintf("policy-%s", pid)
	partition := 0

	conn, err := kafka.DialLeader(s.Context, "tcp", kafkaHost, topic, partition)
	if err != nil {
		return nil, fmt.Errorf("failed to dial leader: %w", err)
	}
	defer conn.Close()

	lastOffset, err := conn.ReadLastOffset()
	if err != nil {
		return nil, fmt.Errorf("failed to read last offset: %w", err)
	}
	if lastOffset == 0 {
		return nil, fmt.Errorf("no policy found")
	}

	// Seek to lastOffset - 1 (the last message)
	if _, err := conn.Seek(lastOffset-1, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to last offset: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	msg, err := conn.ReadMessage(1e6) // max 1MB
	if err != nil {
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	return msg.Value, nil
}
