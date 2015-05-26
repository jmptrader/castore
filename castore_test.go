package castore

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func must_s(s string, err error) string {
	if err != nil {
		panic(err)
	}
	return s
}

func TestSimpleGetPut(t *testing.T) {
	tdir := must_s(ioutil.TempDir("", "castore-test-1"))
	defer os.RemoveAll(tdir)

	s, err := New(Options{
		BasePath: tdir,
	})
	assert.NoError(t, err)

	const (
		TEST_KEY   = "c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2"
		TEST_VALUE = "foobar"
	)

	// Test round-trip value
	key, err := s.Put(strings.NewReader(TEST_VALUE))
	assert.NoError(t, err)
	assert.Equal(t, TEST_KEY, key)

	r, err := s.Get(TEST_KEY)
	assert.NoError(t, err)

	data, err := ioutil.ReadAll(r)
	r.Close()
	assert.NoError(t, err)

	assert.Equal(t, []byte(TEST_VALUE), data)

	// Test non-existent value.
	r, err = s.Get("bad-key")
	assert.NoError(t, err)
	assert.Nil(t, r)

	// Test that the default TransformFunction is the 'flat' one, and that the
	// file exists.
	expectedPath := filepath.Join(tdir, TEST_KEY)
	f, err := os.Open(expectedPath)
	assert.NoError(t, err)
	f.Close()
}

func TestMaxSize(t *testing.T) {
	tdir := must_s(ioutil.TempDir("", "castore-test-2"))
	defer os.RemoveAll(tdir)

	const TEST_LIMIT = 1 * 1024

	s, err := New(Options{
		BasePath: tdir,
		MaxSize:  TEST_LIMIT,
	})
	assert.NoError(t, err)

	// Try writing an infinite amount of data to the store.
	key, err := s.Put(infiniteReader{})
	assert.Equal(t, ErrSizeExceeded, err)
	assert.Equal(t, "", key)

	// A write with the exact size of the limit should succeed
	r := io.LimitReader(infiniteReader{'A'}, TEST_LIMIT-1)
	key, err = s.Put(r)
	assert.NoError(t, err)

	// Note: perl -e "print 'A'x1023" | sha256sum
	assert.Equal(t, "fc189cc673eef6d7ecee4da629f1ed1386479b238dba2ba444e1c7cdde5419b6", key)
}

func TestBadOpts(t *testing.T) {
	_, err := New(Options{
		BasePath: "",
	})
	assert.Equal(t, ErrNoBasePath, err)
}

func TestPutBytes(t *testing.T) {
	tdir := must_s(ioutil.TempDir("", "castore-test-3"))
	defer os.RemoveAll(tdir)

	s, err := New(Options{
		BasePath: tdir,
	})
	assert.NoError(t, err)

	const (
		TEST_KEY   = "c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2"
		TEST_VALUE = "foobar"
	)

	key, err := s.PutBytes([]byte(TEST_VALUE))
	assert.NoError(t, err)
	assert.Equal(t, TEST_KEY, key)

	r, err := s.Get(TEST_KEY)
	assert.NoError(t, err)

	data, err := ioutil.ReadAll(r)
	r.Close()
	assert.NoError(t, err)

	assert.Equal(t, []byte(TEST_VALUE), data)
}
