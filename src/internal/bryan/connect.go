package bryan

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"
)

type GRPCClient struct {
	address   string
	machineID string
	keys      *KeyPair
	messages  chan TaskAssignment
	done      chan struct{}
}

func NewGRPCClient(address string, keysDir string) (*GRPCClient, error) {
	keys, err := LoadOrGenerateKeys(keysDir)
	if err != nil {
		return nil, fmt.Errorf("load keys: %w", err)
	}

	return &GRPCClient{
		address:  address,
		keys:     keys,
		messages: make(chan TaskAssignment, 10),
		done:     make(chan struct{}),
	}, nil
}

func (c *GRPCClient) Connect(ctx context.Context, machineID string, repos []string) error {
	c.machineID = machineID
	// TODO: Establish gRPC connection using generated proto stubs
	// For now, this is a placeholder that will be wired up when proto generation is configured
	log.Printf("bryan: connecting to %s as %s with %d repos", c.address, machineID, len(repos))
	return fmt.Errorf("gRPC connection not yet implemented — proto generation required")
}

func (c *GRPCClient) UpdateTaskStatus(taskID string, status TaskStatus, msg string) error {
	log.Printf("bryan: update task %s status=%d msg=%s", taskID, status, msg)
	return nil
}

func (c *GRPCClient) GetFeatureContext(featureID string) (*FeatureContext, error) {
	return nil, fmt.Errorf("not connected")
}

func (c *GRPCClient) RequestNextTask() error {
	return fmt.Errorf("not connected")
}

func (c *GRPCClient) LogStream(ctx context.Context) (LogSender, error) {
	return nil, fmt.Errorf("not connected")
}

func (c *GRPCClient) ReportUsage(ctx context.Context, report UsageReport) error {
	return fmt.Errorf("not connected")
}

func (c *GRPCClient) SyncMemory(repoURL string, content string) error {
	return fmt.Errorf("not connected")
}

func (c *GRPCClient) Messages() <-chan TaskAssignment {
	return c.messages
}

func (c *GRPCClient) Close() error {
	close(c.done)
	return nil
}

func reconnectBackoff(attempt int) time.Duration {
	base := time.Second
	maxDelay := 30 * time.Second
	delay := time.Duration(float64(base) * math.Pow(2, float64(attempt)))
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}
