package toolkit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTools_RandomString(t *testing.T) {
	var testTools Tools
	s := testTools.RandomString(10)
	assert.Equal(t, 10, len(s))
}
