//go:build !darwin || !cgo

package credentials

type unsupportedBackend struct{}

func New() Backend                                    { return unsupportedBackend{} }
func NewNonInteractive() Backend                      { return unsupportedBackend{} }
func (unsupportedBackend) Get(string) ([]byte, error) { return nil, ErrUnsupported }
func (unsupportedBackend) Put(string, []byte) error   { return ErrUnsupported }
func (unsupportedBackend) Delete(string) error        { return ErrUnsupported }
