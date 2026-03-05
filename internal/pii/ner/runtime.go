//go:build darwin || linux || freebsd || netbsd

package ner

import (
	"context"
	"fmt"

	ort "github.com/shota3506/onnxruntime-purego/onnxruntime"
)

// Runtime wraps an ONNX Runtime session for NER inference.
type Runtime struct {
	rt      *ort.Runtime
	env     *ort.Env
	session *ort.Session
}

// NewRuntime loads the ONNX Runtime shared library and model.
func NewRuntime(paths *Paths) (*Runtime, error) {
	rt, err := ort.NewRuntime(paths.RuntimeLib, 23)
	if err != nil {
		return nil, fmt.Errorf("load onnxruntime: %w", err)
	}

	env, err := rt.NewEnv("hs-ner", ort.LoggingLevelWarning)
	if err != nil {
		rt.Close()
		return nil, fmt.Errorf("create env: %w", err)
	}

	session, err := rt.NewSession(env, paths.ModelONNX, &ort.SessionOptions{
		IntraOpNumThreads: 1,
	})
	if err != nil {
		env.Close()
		rt.Close()
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &Runtime{rt: rt, env: env, session: session}, nil
}

// Run executes inference on the given input_ids and attention_mask.
// Returns logits as [seqLen][numLabels].
func (r *Runtime) Run(inputIDs, attentionMask []int64) ([][]float32, error) {
	seqLen := int64(len(inputIDs))

	idsTensor, err := ort.NewTensorValue(r.rt, inputIDs, []int64{1, seqLen})
	if err != nil {
		return nil, fmt.Errorf("input_ids tensor: %w", err)
	}
	defer idsTensor.Close()

	maskTensor, err := ort.NewTensorValue(r.rt, attentionMask, []int64{1, seqLen})
	if err != nil {
		return nil, fmt.Errorf("attention_mask tensor: %w", err)
	}
	defer maskTensor.Close()

	outputs, err := r.session.Run(context.Background(), map[string]*ort.Value{
		"input_ids":      idsTensor,
		"attention_mask": maskTensor,
	})
	if err != nil {
		return nil, fmt.Errorf("session run: %w", err)
	}

	logitsVal, ok := outputs["logits"]
	if !ok {
		return nil, fmt.Errorf("no 'logits' output")
	}
	defer logitsVal.Close()

	raw, _, err := ort.GetTensorData[float32](logitsVal)
	if err != nil {
		return nil, fmt.Errorf("get logits data: %w", err)
	}

	numLabels := len(raw) / int(seqLen)
	if numLabels == 0 {
		return nil, fmt.Errorf("invalid logits shape")
	}

	logits := make([][]float32, seqLen)
	for i := range logits {
		start := i * numLabels
		logits[i] = raw[start : start+numLabels]
	}
	return logits, nil
}

// Close releases ONNX Runtime resources.
func (r *Runtime) Close() {
	if r.session != nil {
		r.session.Close()
	}
	if r.env != nil {
		r.env.Close()
	}
	if r.rt != nil {
		r.rt.Close()
	}
}
