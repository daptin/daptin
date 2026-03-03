package resource

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/artpar/api2go/v2"
	"github.com/buraksezer/olric"
	jsoniter "github.com/json-iterator/go"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
	"strings"
	"sync"
)

// EventWorkerPool manages event publishing workers
type EventWorkerPool struct {
	workers    chan struct{}
	eventQueue chan EventJob
	wg         sync.WaitGroup
	shutdown   chan struct{}
	metrics    EventMetrics
}

// EventJob represents an event publishing job
type EventJob struct {
	topic     *olric.PubSub
	tableName string
	message   WsOutMessage
}

// EventMetrics tracks event publishing metrics
type EventMetrics struct {
	published uint64
	dropped   uint64
	errors    uint64
	mu        sync.RWMutex
}

var (
	globalEventPool *EventWorkerPool
	eventPoolOnce   sync.Once
)

// GetEventWorkerPool returns the global event worker pool (singleton)
func GetEventWorkerPool() *EventWorkerPool {
	eventPoolOnce.Do(func() {
		poolSize := 50    // Default
		queueSize := 1000 // Default

		if val := os.Getenv("DAPTIN_EVENT_WORKER_POOL_SIZE"); val != "" {
			if size, err := strconv.Atoi(val); err == nil {
				poolSize = size
			}
		}

		if val := os.Getenv("DAPTIN_EVENT_QUEUE_SIZE"); val != "" {
			if size, err := strconv.Atoi(val); err == nil {
				queueSize = size
			}
		}

		globalEventPool = &EventWorkerPool{
			workers:    make(chan struct{}, poolSize),
			eventQueue: make(chan EventJob, queueSize),
			shutdown:   make(chan struct{}),
		}

		// Start workers
		for i := 0; i < poolSize; i++ {
			globalEventPool.wg.Add(1)
			go globalEventPool.worker()
		}

		log.Infof("Event worker pool initialized with %d workers and queue size %d", poolSize, queueSize)
	})

	return globalEventPool
}

// worker processes events from the queue
func (p *EventWorkerPool) worker() {
	defer p.wg.Done()

	for {
		select {
		case job := <-p.eventQueue:
			p.processEvent(job)
		case <-p.shutdown:
			return
		}
	}
}

// processEvent publishes an event
func (p *EventWorkerPool) processEvent(job EventJob) {
	_, err := job.topic.Publish(context.Background(), job.tableName, job.message)
	if err != nil {
		p.metrics.mu.Lock()
		p.metrics.errors++
		p.metrics.mu.Unlock()
		log.Errorf("Failed to publish %s event: %v", job.message.Event, err)
	} else {
		p.metrics.mu.Lock()
		p.metrics.published++
		p.metrics.mu.Unlock()
	}
}

// PublishEvent queues an event for publishing
func (p *EventWorkerPool) PublishEvent(topic *olric.PubSub, tableName string, message WsOutMessage) {
	job := EventJob{
		topic:     topic,
		tableName: tableName,
		message:   message,
	}

	select {
	case p.eventQueue <- job:
		// Successfully queued
	default:
		// Queue full, drop the event
		p.metrics.mu.Lock()
		p.metrics.dropped++
		p.metrics.mu.Unlock()
		log.Warnf("Event queue full, dropping %s event for %s", message.Event, tableName)
	}
}

type eventHandlerMiddleware struct {
	dtopicMap *map[string]*olric.PubSub
	cruds     *map[string]*DbResource
}

func (pc eventHandlerMiddleware) String() string {
	return "EventGenerator"
}

// WsOutMessage is the single server→client message type on the WebSocket wire.
// The Type field distinguishes response/event/session messages.
type WsOutMessage struct {
	// Common
	Type string `json:"type"` // "response" | "event" | "session" | "pong"

	// Response fields (type == "response")
	Id     string `json:"id,omitempty"`     // echo of client request id
	Method string `json:"method,omitempty"` // echo of client method
	Ok     *bool  `json:"ok,omitempty"`     // success/failure
	Error  string `json:"error,omitempty"`  // error message when ok=false

	// Event fields (type == "event")
	Topic  string `json:"topic,omitempty"`  // topic name
	Event  string `json:"event,omitempty"`  // "create" | "update" | "delete" | "new-message"
	Source string `json:"source,omitempty"` // "database" or user ref id

	// Session fields (type == "session")
	Status string `json:"status,omitempty"` // "open"

	// Shared data payload — proper JSON object, not base64 bytes
	Data jsoniter.RawMessage `json:"data,omitempty"`
}

// MarshalBinary encodes the struct into binary format for Olric PubSub transport.
func (e WsOutMessage) MarshalBinary() (data []byte, err error) {
	buffer := new(bytes.Buffer)

	if err := encodeString(buffer, e.Type); err != nil {
		return nil, err
	}
	if err := encodeString(buffer, e.Topic); err != nil {
		return nil, err
	}
	if err := encodeString(buffer, e.Event); err != nil {
		return nil, err
	}
	if err := encodeString(buffer, e.Source); err != nil {
		return nil, err
	}
	if err := encodeString(buffer, string(e.Data)); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// UnmarshalBinary decodes the data from Olric PubSub binary transport.
func (e *WsOutMessage) UnmarshalBinary(data []byte) error {
	buffer := bytes.NewBuffer(data)

	if v, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.Type = v
	}

	if v, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.Topic = v
	}

	if v, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.Event = v
	}

	if v, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.Source = v
	}

	if v, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.Data = jsoniter.RawMessage(v)
		return nil
	}
}

// Helper functions to encode and decode strings
func encodeString(buffer *bytes.Buffer, s string) error {
	length := int32(len(s))
	if err := binary.Write(buffer, binary.BigEndian, length); err != nil {
		return err
	}
	if _, err := buffer.WriteString(s); err != nil {
		return err
	}
	return nil
}

func decodeString(buffer *bytes.Buffer) (string, error) {
	var length int32
	if err := binary.Read(buffer, binary.BigEndian, &length); err != nil {
		return "", err
	}
	data := make([]byte, length)
	if _, err := buffer.Read(data); err != nil {
		return "", err
	}
	return string(data), nil
}

func (pc *eventHandlerMiddleware) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}, transaction *sqlx.Tx) ([]map[string]interface{}, error) {

	tableName := dr.model.GetTableName()
	topic := (*pc.dtopicMap)[tableName]
	if topic == nil {
		return results, nil
	}

	switch strings.ToLower(req.PlainRequest.Method) {
	case "get":
		break
	case "post":
		messageBytes, err := json.Marshal(results[0])
		if err != nil {
			log.Errorf("Failed to serialize create message: %v", err)
		} else {
			GetEventWorkerPool().PublishEvent(topic, tableName, WsOutMessage{
				Type:   "event",
				Topic:  dr.model.GetTableName(),
				Event:  "create",
				Source: "database",
				Data:   messageBytes,
			})
		}
		break
	case "delete":
		messageBytes, err := json.Marshal(results[0])
		if err != nil {
			log.Errorf("Failed to serialize delete message: %v", err)
		} else {
			GetEventWorkerPool().PublishEvent(topic, tableName, WsOutMessage{
				Type:   "event",
				Topic:  dr.model.GetTableName(),
				Event:  "delete",
				Source: "database",
				Data:   messageBytes,
			})
		}
		break
	case "patch":
		messageBytes, err := json.Marshal(results[0])
		if err != nil {
			log.Errorf("Failed to serialize update message: %v", err)
		} else {
			GetEventWorkerPool().PublishEvent(topic, tableName, WsOutMessage{
				Type:   "event",
				Topic:  dr.model.GetTableName(),
				Event:  "update",
				Source: "database",
				Data:   messageBytes,
			})
		}
		break
	default:
		log.Errorf("Invalid method: %v", req.PlainRequest.Method)
	}

	return results, nil

}

func (pc *eventHandlerMiddleware) InterceptBefore(dr *DbResource, req *api2go.Request, objects []map[string]interface{}, transaction *sqlx.Tx) ([]map[string]interface{}, error) {

	reqmethod := req.PlainRequest.Method
	//log.Printf("Generate events for objects", reqmethod)
	switch reqmethod {
	case "GET":
		break
	case "POST":
		break
	case "DELETE":
		break
	case "PATCH":
		break
	default:
		log.Errorf("Invalid method: %v", reqmethod)
	}

	//currentUserId := context.Get(req.PlainRequest, "user_id").(string)
	//currentUserGroupId := context.Get(req.PlainRequest, "usergroup_id").([]string)

	return objects, nil

}
