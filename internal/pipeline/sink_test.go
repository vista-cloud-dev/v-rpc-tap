package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidate_RequiresGovCloudPartition(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		ok   bool
	}{
		{"govcloud west", Config{Bucket: "b", Partition: "aws-us-gov", Region: "us-gov-west-1"}, true},
		{"govcloud east", Config{Bucket: "b", Partition: "aws-us-gov", Region: "us-gov-east-1"}, true},
		{"commercial partition rejected", Config{Bucket: "b", Partition: "aws", Region: "us-east-1"}, false},
		{"china partition rejected", Config{Bucket: "b", Partition: "aws-cn", Region: "cn-north-1"}, false},
		{"gov partition but commercial region rejected", Config{Bucket: "b", Partition: "aws-us-gov", Region: "us-east-1"}, false},
		{"missing bucket rejected", Config{Partition: "aws-us-gov", Region: "us-gov-west-1"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.cfg.Validate()
			if c.ok && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
			if !c.ok && err == nil {
				t.Errorf("Validate() = nil, want a GovCloud/partition error")
			}
		})
	}
}

func TestMemSink_RecordsPuts(t *testing.T) {
	s := NewMemSink()
	if err := s.Put(context.Background(), "k1", []byte("body1")); err != nil {
		t.Fatal(err)
	}
	if got := string(s.Objects["k1"]); got != "body1" {
		t.Errorf("MemSink[k1] = %q, want body1", got)
	}
}

func TestFileSink_WritesUnderRoot(t *testing.T) {
	dir := t.TempDir()
	s := NewFileSink(dir)
	if err := s.Put(context.Background(), "engine=ydb/window=0000000001.ldjson", []byte("x\n")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "engine=ydb/window=0000000001.ldjson"))
	if err != nil {
		t.Fatalf("object not written under root: %v", err)
	}
	if string(got) != "x\n" {
		t.Errorf("object body = %q, want x\\n", got)
	}
}
