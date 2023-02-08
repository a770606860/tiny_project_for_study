package gebcache

import (
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

func TestHashing(t *testing.T) {
	hash := newCSHPicker(3, func(key []byte) uint32 {
		i, _ := strconv.Atoi(string(key))
		return uint32(i)
	})
	hash.Add("6", "4", "2")
	testCases := map[string]string{
		"2":   "2",
		"11":  "2",
		"23":  "4",
		"100": "2",
		"15":  "6",
	}
	for k, v := range testCases {
		assert.Equal(t, v, hash.Pick(k))
	}
}
