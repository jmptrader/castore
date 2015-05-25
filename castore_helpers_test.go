package castore

// Helpers for testing

type infiniteReader struct {
	Ch byte
}

func (r infiniteReader) Read(b []byte) (int, error) {
	for i := 0; i < len(b); i++ {
		b[i] = r.Ch
	}
	return len(b), nil
}
