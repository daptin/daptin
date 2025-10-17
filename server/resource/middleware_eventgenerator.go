package resource

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/artpar/api2go/v2"
	"github.com/buraksezer/olric"
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
	message   EventMessage
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
		log.Errorf("Failed to publish %s event: %v", job.message.EventType, err)
	} else {
		p.metrics.mu.Lock()
		p.metrics.published++
		p.metrics.mu.Unlock()
	}
}

// PublishEvent queues an event for publishing
func (p *EventWorkerPool) PublishEvent(topic *olric.PubSub, tableName string, message EventMessage) {
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
		log.Warnf("Event queue full, dropping %s event for %s", message.EventType, tableName)
	}
}

type eventHandlerMiddleware struct {
	dtopicMap *map[string]*olric.PubSub
	cruds     *map[string]*DbResource
}

func (pc eventHandlerMiddleware) String() string {
	return "EventGenerator"
}

type EventMessage struct {
	MessageSource string
	EventType     string
	ObjectType    string
	EventData     []byte
}

// MarshalBinary encodes the struct into binary format manually
func (e EventMessage) MarshalBinary() (data []byte, err error) {
	buffer := new(bytes.Buffer)

	// Encode MessageSource
	if err := encodeString(buffer, e.MessageSource); err != nil {
		return nil, err
	}

	// Encode EventType
	if err := encodeString(buffer, e.EventType); err != nil {
		return nil, err
	}

	// Encode ObjectType
	if err := encodeString(buffer, e.ObjectType); err != nil {
		return nil, err
	}

	// Simplified handling for EventData: encoding just the length (this should be replaced with actual data encoding logic)
	jsonStr := string(e.EventData)
	if err := encodeString(buffer, jsonStr); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// UnmarshalBinary decodes the data into the struct using manual binary decoding
func (e *EventMessage) UnmarshalBinary(data []byte) error {
	buffer := bytes.NewBuffer(data)

	// Decode MessageSource
	if msgSource, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.MessageSource = msgSource
	}

	// Decode EventType
	if eventType, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.EventType = eventType
	}

	// Decode ObjectType
	if objectType, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.ObjectType = objectType
	}

	// Assume EventData is just the count of items (real logic needed to parse actual data)
	if eventDataJson, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.EventData = []byte(eventDataJson)
		return err
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
			GetEventWorkerPool().PublishEvent(topic, tableName, EventMessage{
				MessageSource: "database",
				EventType:     "create",
				ObjectType:    dr.model.GetTableName(),
				EventData:     messageBytes,
			})
		}
		break
	case "delete":
		messageBytes, err := json.Marshal(results[0])
		if err != nil {
			log.Errorf("Failed to serialize delete message: %v", err)
		} else {
			GetEventWorkerPool().PublishEvent(topic, tableName, EventMessage{
				MessageSource: "database",
				EventType:     "delete",
				ObjectType:    dr.model.GetTableName(),
				EventData:     messageBytes,
			})
		}
		break
	case "patch":
		messageBytes, err := json.Marshal(results[0])
		if err != nil {
			log.Errorf("Failed to serialize update message: %v", err)
		} else {
			GetEventWorkerPool().PublishEvent(topic, tableName, EventMessage{
				MessageSource: "database",
				EventType:     "update",
				ObjectType:    dr.model.GetTableName(),
				EventData:     messageBytes,
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
