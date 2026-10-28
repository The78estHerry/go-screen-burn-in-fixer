//go:build !windows

package display

func platformDisplays() ([]Display, error) {
	return nil, ErrUnsupported
}
