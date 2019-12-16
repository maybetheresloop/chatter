package chatter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMakeJoinMessage(t *testing.T) {
	keys := make([]ChannelKey, 0)
	assert.Equal(t, "JOIN 0\r\n", makeJoinMessage(keys))

	keys = []ChannelKey{
		{"channel1", "key1"},
		{"channel2", ""},
		{"channel3", ""},
		{"channel4", "key4"},
		{"channel5", "key5"}}

	assert.Equal(t, "JOIN channel1,channel2,channel3,channel4,channel5 key1,,,key4,key5\r\n", makeJoinMessage(keys))
}
