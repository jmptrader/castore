package castore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlatTransformFunc(t *testing.T) {
	assert.Equal(t, []string{}, FlatTransformFunc("abcdef"))
}

func TestDepthTransformFunc(t *testing.T) {
	d1 := DepthTransformFunc(1)

	assert.Equal(t, []string{"ab"}, d1("abcdef"))
	assert.Equal(t, []string{"ab"}, d1("ab"))
	assert.Panics(t, func() {
		d1("a")
	})

	d2 := DepthTransformFunc(2)
	assert.Equal(t, []string{"ab", "cd"}, d2("abcdef"))
	assert.Equal(t, []string{"ab", "cd"}, d2("abcd"))
	assert.Panics(t, func() {
		d2("abc")
	})
}
