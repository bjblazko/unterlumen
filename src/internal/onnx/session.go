package onnx

import (
	"fmt"
	"unsafe"
)

// Tensor is a float32 tensor: Data holds the product of Shape's values, row-major.
type Tensor struct {
	Shape []int64
	Data  []float32
}

// Session is a loaded model. Run may be called from several goroutines at once.
type Session struct {
	rt       *Runtime
	ptr      unsafe.Pointer
	inNames  []*byte // NUL-terminated, kept alive for Run
	outNames []*byte
}

// NewSession loads a model from its bytes. threads limits the threads one
// Run uses; 0 lets ONNX Runtime choose.
func (r *Runtime) NewSession(model []byte, threads int) (*Session, error) {
	var opts unsafe.Pointer
	if err := r.status(r.api.createSessionOptions(unsafe.Pointer(&opts))); err != nil {
		return nil, err
	}
	defer r.api.releaseSessionOptions(opts)
	if err := r.configure(opts, threads); err != nil {
		return nil, err
	}
	s := &Session{rt: r}
	err := r.status(r.api.createSessionFromArray(r.env, unsafe.Pointer(&model[0]), uintptr(len(model)), opts, unsafe.Pointer(&s.ptr)))
	keepAlive(model)
	if err != nil {
		return nil, err
	}
	if err := s.readNames(); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (r *Runtime) configure(opts unsafe.Pointer, threads int) error {
	if err := r.status(r.api.setGraphOptimization(opts, ortEnableAllOptimizations)); err != nil {
		return err
	}
	if threads > 0 {
		if err := r.status(r.api.setIntraOpNumThreads(opts, int32(threads))); err != nil {
			return err
		}
	}
	return r.status(r.api.setInterOpNumThreads(opts, 1))
}

// readNames copies the model's input and output names into Go memory.
func (s *Session) readNames() error {
	a := s.rt.api
	var alloc unsafe.Pointer
	if err := s.rt.status(a.getDefaultAllocator(unsafe.Pointer(&alloc))); err != nil {
		return err
	}
	read := func(count func(unsafe.Pointer, unsafe.Pointer) unsafe.Pointer,
		name func(unsafe.Pointer, uintptr, unsafe.Pointer, unsafe.Pointer) unsafe.Pointer) ([]*byte, error) {
		var n uintptr
		if err := s.rt.status(count(s.ptr, unsafe.Pointer(&n))); err != nil {
			return nil, err
		}
		names := make([]*byte, n)
		for i := range names {
			var p *byte
			if err := s.rt.status(name(s.ptr, uintptr(i), alloc, unsafe.Pointer(&p))); err != nil {
				return nil, err
			}
			b := append([]byte(goString(p)), 0)
			a.allocatorFree(alloc, unsafe.Pointer(p))
			names[i] = &b[0]
		}
		return names, nil
	}
	var err error
	if s.inNames, err = read(a.sessionGetInputCount, a.sessionGetInputName); err != nil {
		return err
	}
	s.outNames, err = read(a.sessionGetOutputCount, a.sessionGetOutputName)
	return err
}

// InputNames and OutputNames are the model's, in its order.
func (s *Session) InputNames() []string  { return names(s.inNames) }
func (s *Session) OutputNames() []string { return names(s.outNames) }

func names(ps []*byte) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = goString(p)
	}
	return out
}

// Run runs the model on inputs, one per input name and in that order, and
// returns every output in the model's order.
func (s *Session) Run(inputs ...Tensor) ([]Tensor, error) {
	if len(inputs) != len(s.inNames) {
		return nil, fmt.Errorf("the model takes %d inputs, not %d", len(s.inNames), len(inputs))
	}
	in := make([]unsafe.Pointer, len(inputs))
	defer s.release(in)
	for i, t := range inputs {
		v, err := s.rt.newTensor(t)
		if err != nil {
			return nil, err
		}
		in[i] = v
	}
	out := make([]unsafe.Pointer, len(s.outNames))
	defer s.release(out)
	err := s.rt.status(s.rt.api.run(s.ptr, nil,
		unsafe.Pointer(&s.inNames[0]), unsafe.Pointer(&in[0]), uintptr(len(in)),
		unsafe.Pointer(&s.outNames[0]), uintptr(len(out)), unsafe.Pointer(&out[0])))
	keepAlive(inputs, s.inNames, s.outNames)
	if err != nil {
		return nil, err
	}
	results := make([]Tensor, len(out))
	for i, v := range out {
		if results[i], err = s.rt.readTensor(v); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (s *Session) release(values []unsafe.Pointer) {
	for _, v := range values {
		if v != nil {
			s.rt.api.releaseValue(v)
		}
	}
}

// Close releases the model.
func (s *Session) Close() {
	if s.ptr != nil {
		s.rt.api.releaseSession(s.ptr)
		s.ptr = nil
	}
}

// newTensor wraps t's data without copying it; t must outlive the value.
func (r *Runtime) newTensor(t Tensor) (unsafe.Pointer, error) {
	n := int64(1)
	for _, d := range t.Shape {
		n *= d
	}
	if n != int64(len(t.Data)) || n == 0 {
		return nil, fmt.Errorf("a tensor of shape %v cannot hold %d values", t.Shape, len(t.Data))
	}
	var v unsafe.Pointer
	err := r.status(r.api.createTensorWithData(r.mem, unsafe.Pointer(&t.Data[0]), uintptr(len(t.Data)*4),
		unsafe.Pointer(&t.Shape[0]), uintptr(len(t.Shape)), ortTensorElementFloat, unsafe.Pointer(&v)))
	return v, err
}

// readTensor copies an output value into Go memory.
func (r *Runtime) readTensor(v unsafe.Pointer) (Tensor, error) {
	var info unsafe.Pointer
	if err := r.status(r.api.getTensorTypeAndShape(v, unsafe.Pointer(&info))); err != nil {
		return Tensor{}, err
	}
	defer r.api.releaseTensorTypeAndShape(info)
	var dims uintptr
	if err := r.status(r.api.getDimensionsCount(info, unsafe.Pointer(&dims))); err != nil {
		return Tensor{}, err
	}
	shape := make([]int64, dims)
	if dims > 0 {
		if err := r.status(r.api.getDimensions(info, unsafe.Pointer(&shape[0]), dims)); err != nil {
			return Tensor{}, err
		}
	}
	n := int64(1)
	for _, d := range shape {
		n *= d
	}
	var p unsafe.Pointer
	if err := r.status(r.api.getTensorMutableData(v, unsafe.Pointer(&p))); err != nil {
		return Tensor{}, err
	}
	data := make([]float32, n)
	if n > 0 {
		copy(data, unsafe.Slice((*float32)(p), n))
	}
	return Tensor{Shape: shape, Data: data}, nil
}
