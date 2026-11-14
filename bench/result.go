package bench

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// Result holds one run of one scenario variant on one client. Each run
// writes one Result file. Java writes the same JSON fields.
type Result struct {
	Scenario string `json:"scenario"`
	Client   string `json:"client"`
	Variant  string `json:"variant"`
	Config   Config `json:"config"`

	// Throughput holds the rate of a throughput scenario. Nil otherwise.
	Throughput *Throughput `json:"throughput,omitempty"`

	// Latency holds the percentiles of a ping-pong, cluster or fixed-rate
	// scenario, and the histogram of a ping-pong or cluster one. Nil
	// otherwise.
	Latency *Latency `json:"latency,omitempty"`

	// FixedRate holds the counts and the histogram of a fixed-rate scenario.
	// Nil otherwise.
	FixedRate *FixedRate `json:"fixed_rate,omitempty"`

	// GoRuntime holds the Go counters. Nil for the Java client.
	GoRuntime *GoRuntime `json:"go_runtime,omitempty"`

	// JVMRuntime holds the JVM counters. Nil for the Go clients.
	JVMRuntime *JVMRuntime `json:"jvm_runtime,omitempty"`
}

// WriteJSON writes v as indented JSON to path. An empty path writes to
// stdout.
func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	data = append(data, '\n')
	if path == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			return fmt.Errorf("write json to stdout: %w", err)
		}
		return nil
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}

// ReadJSON decodes the JSON file at path into v. ReadJSON ignores unknown
// fields, so it reads a file that an older commit wrote. ReadJSON rejects
// data after the first JSON value.
func ReadJSON(path string, v any) error {
	return readJSON(path, v, false)
}

// ReadJSONStrict decodes like ReadJSON. ReadJSONStrict also rejects unknown
// fields.
func ReadJSONStrict(path string, v any) error {
	return readJSON(path, v, true)
}

func readJSON(path string, v any, strict bool) (err error) {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("read json: %w", err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	decoder := json.NewDecoder(file)
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("decode json %s: %w", path, err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode json %s: data after the first value", path)
	}
	return nil
}
