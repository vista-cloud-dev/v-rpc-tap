// Package pipeline is the read-only drain → object-store → committrim-after-ack flow
// (D12, at-least-once): it pulls correlated records off the engine ring via the host
// controller, ships one LDJSON object per drain window to a Sink, and only after the
// Sink acknowledges the write does it commit the engine-side trim. Nothing is deleted
// from the ring until its bytes are durable, so a crash between ship and ack costs a
// re-drain (de-duped on (inc,job,seq)), never data loss.
//
// The production Sink is S3 in the GovCloud partition — RPC-traffic egress (incl. PHI)
// is authorized only to FedRAMP-High US GovCloud (U4). That real S3 adapter (aws-sdk-go
// + live creds) is DEFERRED; MemSink and FileSink make the whole flow testable and
// locally runnable now (FileSink also stands in for the local MinIO lab).
package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sink stores one drained object durably under a key. At-least-once: the pipeline
// commits the engine-side trim only after Put returns nil.
type Sink interface {
	Put(ctx context.Context, key string, body []byte) error
}

// Config pins the object store to the GovCloud partition (U4).
type Config struct {
	Bucket    string // destination bucket
	Region    string // a GovCloud region: us-gov-west-1 | us-gov-east-1
	Partition string // must be "aws-us-gov"
	Prefix    string // optional key prefix within the bucket
}

// govPartition is the only AWS partition RPC traffic may egress to (U4).
const govPartition = "aws-us-gov"

// Validate enforces the GovCloud pin: the egress ATO covers only the GovCloud
// partition, so a commercial ("aws") or China ("aws-cn") target is refused here rather
// than discovered at deploy time.
func (c Config) Validate() error {
	if c.Bucket == "" {
		return fmt.Errorf("pipeline config: bucket is required")
	}
	if c.Partition != govPartition {
		return fmt.Errorf("pipeline config: partition %q not allowed — RPC egress is authorized only to %q (U4)", c.Partition, govPartition)
	}
	if !strings.HasPrefix(c.Region, "us-gov-") {
		return fmt.Errorf("pipeline config: region %q is not a GovCloud region (want us-gov-*)", c.Region)
	}
	return nil
}

// MemSink is an in-memory Sink for tests.
type MemSink struct {
	Objects map[string][]byte
}

// NewMemSink returns an empty in-memory sink.
func NewMemSink() *MemSink { return &MemSink{Objects: map[string][]byte{}} }

// Put stores body under key.
func (s *MemSink) Put(_ context.Context, key string, body []byte) error {
	s.Objects[key] = body
	return nil
}

// FileSink writes objects under a local directory root — usable now for the local lab
// (and a stand-in for an S3-compatible MinIO endpoint).
type FileSink struct {
	root string
}

// NewFileSink returns a sink rooted at dir.
func NewFileSink(dir string) *FileSink { return &FileSink{root: dir} }

// Put writes body to <root>/<key>, creating parent directories.
func (s *FileSink) Put(_ context.Context, key string, body []byte) error {
	path := filepath.Join(s.root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("filesink mkdir: %w", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return fmt.Errorf("filesink write %s: %w", key, err)
	}
	return nil
}
