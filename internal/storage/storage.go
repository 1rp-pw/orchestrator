package storage

import (
	"context"
	"encoding/json"
	"fmt"
	policymodel "github.com/1rp-pw/orchestrator/internal/policy"
	"github.com/bugfixes/go-bugfixes/logs"
	kafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/google/uuid"
	ConfigBuilder "github.com/keloran/go-config"
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

	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": s.Config.ProjectProperties["kafka_host"],
	})
	if err != nil {
		_ = logs.Errorf("failed to create producer: %v", err)
		http.Error(w, "failed to create policy", http.StatusInternalServerError)
	}
	defer p.Close()
	go func() {
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					_ = logs.Errorf("Delivery failed: %v", ev.TopicPartition)
				}
			}
		}
	}()

	data, err := json.Marshal(policy)
	if err != nil {
		_ = logs.Errorf("failed to create policy: %v", err)
		http.Error(w, "failed to create policy", http.StatusInternalServerError)
	}

	topic := fmt.Sprintf("policy-%s", uid.String())
	if err := p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: 0,
		},
		Value: data,
	}, nil); err != nil {
		_ = logs.Errorf("failed to create policy: %v", err)
		http.Error(w, "failed to create policy", http.StatusInternalServerError)
	}

	if err := json.NewEncoder(w).Encode(&policy); err != nil {
		_ = logs.Errorf("failed to create policy: %v", err)
		http.Error(w, "failed to create policy", http.StatusInternalServerError)
	}
	p.Flush(15 * 1000)
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

	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": s.Config.ProjectProperties["kafka_host"],
	})
	if err != nil {
		_ = logs.Errorf("failed to create producer: %v", err)
		http.Error(w, "failed to create policy", http.StatusInternalServerError)
	}
	defer p.Close()
	go func() {
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					_ = logs.Errorf("Delivery failed: %v", ev.TopicPartition)
				}
			}
		}
	}()

	data, err := json.Marshal(policy)
	if err != nil {
		_ = logs.Errorf("failed to create policy: %v", err)
		http.Error(w, "failed to create policy", http.StatusInternalServerError)
	}

	topic := fmt.Sprintf("policy-%s", policyId)
	if err := p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: 0,
		},
		Value: data,
	}, nil); err != nil {
		_ = logs.Errorf("failed to create policy: %v", err)
		http.Error(w, "failed to create policy", http.StatusInternalServerError)
	}

	if err := json.NewEncoder(w).Encode(&policy); err != nil {
		_ = logs.Errorf("failed to create policy: %v", err)
		http.Error(w, "failed to create policy", http.StatusInternalServerError)
	}
	p.Flush(15 * 1000)
}

func (s *System) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *System) GetPolicy(w http.ResponseWriter, r *http.Request) {
	s.Context = r.Context()

	policyId := r.PathValue("policyId")
	p, err := s.GetLatestPolicyFromStorage(policyId)
	if err != nil {
		_ = logs.Errorf("Failed to get policy from storage: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		_ = logs.Errorf("Failed to get policy from storage: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(p); err != nil {
		_ = logs.Errorf("Failed to write response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *System) GetPolicyFromStorage(policyId string) ([]interface{}, error) {
	policy := fmt.Sprintf("policy-%s", policyId)

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":        s.Config.ProjectProperties["kafka_host"].(string),
		"auto.offset.reset":        "earliest",
		"group.id":                 policy,
		"enable.partition.eof":     true,
		"go.events.channel.enable": false,
	})
	if err != nil {
		return nil, logs.Errorf("Failed to create consumer: %v", err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			_ = logs.Errorf("Failed to close consumer: %v", err)
		}
	}()

	low, high, err := c.QueryWatermarkOffsets(policy, int32(0), 5*1000)
	if err != nil {
		return nil, logs.Errorf("Failed to query watermark offsets: %v", err)
	}
	messageCount := high - low
	if high == low || messageCount == 0 {
		return nil, logs.Error("no policy found")
	}

	if err := c.Assign([]kafka.TopicPartition{{
		Topic:     &policy,
		Partition: 0,
		Offset:    kafka.Offset(low),
	}}); err != nil {
		return nil, logs.Errorf("Failed to assign partition: %v", err)
	}

	var messages []interface{}
	for i := int64(0); i < messageCount; i++ {
		ev := c.Poll(100)
		if ev == nil {
			return nil, logs.Error("Failed to poll partition")
		}

		switch e := ev.(type) {
		case *kafka.Message:
			var message interface{}
			if err := json.Unmarshal(e.Value, &message); err != nil {
				_ = logs.Errorf("Failed to unmarshal message: %v", err)
				break
			}
			messages = append(messages, message)
		case kafka.PartitionEOF:
			i = messageCount
		case kafka.Error:
			return nil, logs.Errorf("Error from partition: %v", e)
		}
	}

	return messages, nil
}

func (s *System) GetLatestPolicyFromStorage(pid string) (interface{}, error) {
	policy := fmt.Sprintf("policy-%s", pid)

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": s.Config.ProjectProperties["kafka_host"].(string),
		"auto.offset.reset": "latest",
		"group.id":          policy,
	})
	if err != nil {
		return nil, logs.Errorf("Failed to create consumer: %v", err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			_ = logs.Errorf("Failed to close consumer: %v", err)
		}
	}()

	low, high, err := c.QueryWatermarkOffsets(policy, int32(0), 5*1000)
	if err != nil {
		return nil, logs.Errorf("Failed to query watermark offsets: %v", err)
	}
	if high == low {
		return nil, logs.Error("no policy found")
	}

	if err := c.Assign([]kafka.TopicPartition{{
		Topic:     &policy,
		Partition: 0,
		Offset:    kafka.Offset(high - 1),
	}}); err != nil {
		return nil, logs.Errorf("Failed to assign partition: %v", err)
	}

	ev := c.Poll(100)
	if ev == nil {
		return nil, logs.Error("Failed to poll partition")
	}

	switch e := ev.(type) {
	case *kafka.Message:
		return e.Value, nil
	case kafka.PartitionEOF:
		return nil, logs.Error("Partition EOF")
	case kafka.Error:
		return nil, logs.Errorf("kafka Error %v", e)
	default:
		return nil, logs.Error("Unknown event")
	}
}
