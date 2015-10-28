package castore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// TransformFunction transforms a key into a slice of strings, each of which
// represents a directory where the data will be stored.  For example, if a
// TransformFunction turns "abcdef" into `[]string{"ab", "cd", "ef"}`, the
// final location of the data on-disk will be `<BasePath>/ab/cd/ef/abcdef`.
type TransformFunction func(key string) []string

// Options contains the options that control the behavior of a CAStore.
type Options struct {
	// BasePath is the root directory for storing all data. This must be provided.
	BasePath string

	// Hash is the hash function that will be used to generate keys.  If this is
	// not specified, it will default to crypto/sha256.
	Hash func() hash.Hash

	// Transform is the TransformFunction that will be used for the CAStore.  If
	// this is not specified, it will default to FlatTransformFunc.
	Transform TransformFunction

	// MaxSize specifies the upper limit on the size of values that can be
	// inserted into the CAStore.  If not specified or negative, this will default
	// to 10 MiB.
	MaxSize int64
}

var (
	// ErrSizeExceeded is the error returned when an attempt is made to store data
	// that exceeds the MaxSize specified when creating the CAStore.
	ErrSizeExceeded = errors.New("castore: size exceeded")

	// ErrNoBasePath is the error returned when attempting to construct a CAStore
	// with no BasePath specified.
	ErrNoBasePath = errors.New("castore: base path cannot be empty")
)

// CAStore implements a content-addressable storage for arbitrary inputs.
type CAStore struct {
	opts Options
}

// New will create a new CAStore with the given options.  It will attempt to
// create the directory given in Options.BasePath, and will return an error if
// the directory cannot be created.  No error is returned if the directory
// already exists.
func New(opts Options) (*CAStore, error) {
	if opts.BasePath == "" {
		return nil, ErrNoBasePath
	}

	// Try creating the base path
	err := os.MkdirAll(opts.BasePath, 0700)
	if err != nil {
		return nil, fmt.Errorf("castore: could not create the base path: %s", err)
	}

	// Set default options
	if opts.Hash == nil {
		opts.Hash = sha256.New
	}
	if opts.Transform == nil {
		opts.Transform = FlatTransformFunc
	}
	if opts.MaxSize <= 0 {
		opts.MaxSize = 10 * 1024 * 1024
	}

	// Ready!
	ret := &CAStore{
		opts: opts,
	}
	return ret, nil
}

// copyLimited is a helper function that will copy from an io.Reader to an
// io.Writer, but limited to a certain number of bytes.  It will return the
// number of bytes written, whether we exceeded the limit, and any error.
func (s *CAStore) copyLimited(dst io.Writer, src io.Reader, limit int64) (int64, bool, error) {
	var (
		remaining = limit
		written   int64
		buffer    = make([]byte, 32*1024)
		tooLarge  bool
		err       error
	)
	for {
		buf := buffer

		// Failure case for being too large.
		if remaining <= 0 {
			tooLarge = true
			break
		}

		// Ensure that we don't read more than the remaining size on this iteration.
		if int64(len(buf)) > remaining {
			buf = buf[0:remaining]
		}

		nr, er := src.Read(buf)
		if nr > 0 {
			remaining -= int64(nr)

			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}

		// Success
		if er == io.EOF {
			break
		}

		if er != nil {
			err = er
			break
		}
	}

	return written, tooLarge, err
}

// Put will insert the data from the given io.Reader into the store, and return
// the key that was used to insert
func (s *CAStore) Put(r io.Reader) (string, error) {
	// Create a temporary file to stream the data to.
	tfile, err := ioutil.TempFile("", "castore")
	if err != nil {
		return "", err
	}

	// Create a new instance of the hash.
	hasher := s.opts.Hash()

	// We use a writer that writes to both the temporary file and the hasher.
	w := io.MultiWriter(tfile, hasher)

	// Copy up to the maximum amount of data.
	_, tooLarge, err := s.copyLimited(w, r, s.opts.MaxSize)

	// We're done with our temporary file here, regardless of success/failure.
	tfile.Close()

	// If we're too large, return that.
	if tooLarge {
		os.Remove(tfile.Name())
		return "", ErrSizeExceeded
	}

	// err should be non-nil here if there was an error copying, so we handle it.
	if err != nil {
		os.Remove(tfile.Name())
		return "", err
	}

	// Everything was successful!  Get the final key from our hasher.
	sum := hasher.Sum(nil)
	key := hex.EncodeToString(sum)

	// Ensure the directory exists.
	dirPath := s.transform(key)
	if err = os.MkdirAll(dirPath, 0700); err != nil {
		os.Remove(tfile.Name())
		return "", err
	}

	// Move the file to the directory.
	if err = os.Rename(tfile.Name(), filepath.Join(dirPath, key)); err != nil {
		os.Remove(tfile.Name())
		return "", err
	}

	// All done!
	return key, nil
}

// PutBytes is a helper function to put a byte array into the store.
func (s *CAStore) PutBytes(b []byte) (string, error) {
	return s.Put(bytes.NewReader(b))
}

// PutString is a helper function to put a string into the store.
func (s *CAStore) PutString(val string) (string, error) {
	return s.Put(strings.NewReader(val))
}

// Get will return an io.ReadCloser that represents the data stored with the
// given key.  If the key does not exist in the store, then `nil` will be
// returned instead.
func (s *CAStore) Get(key string) (io.ReadCloser, error) {
	// Try opening the file.
	f, err := os.Open(filepath.Join(s.transform(key), key))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return f, err
}

// Size will return the size of the data stored with the given key.  If the key
// does not exist in the store, then the returned value will be negative.
func (s *CAStore) Size(key string) (int64, error) {
	// Try opening the file.
	inf, err := os.Stat(filepath.Join(s.transform(key), key))
	if os.IsNotExist(err) {
		return -1, nil
	}
	if err != nil {
		return 0, err
	}

	return inf.Size(), nil
}

// transform is a helper function that will take the given key and return the
// containing directory's path on-disk (including the BaseDir).
func (s *CAStore) transform(key string) string {
	dirs := s.opts.Transform(key)
	return filepath.Join(s.opts.BasePath, filepath.Join(dirs...))
}

// FlatTransformFunc will place all files in the same directory.
func FlatTransformFunc(key string) []string {
	return []string{}
}

// DepthTransformFunc will split the input key into the given number of two-
// length strings and use those as directories.  For example, for the input
// key "abcdef", DepthTransformFunc(1) will return:
//     []string{"ab"}
// DepthTransformFunc(2) will return:
//     []string{"ab", "cd"}
// and so on.
func DepthTransformFunc(depth int) TransformFunction {
	return func(key string) []string {
		ret := []string{}
		for i := 0; i < depth; i++ {
			ret = append(ret, key[i*2:(i+1)*2])
		}

		return ret
	}
}
