package firego

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zabawaba99/firetest"
)

func TestUpdate(t *testing.T) {
	t.Parallel()
	var (
		payload = map[string]interface{}{"foo": "bar"}
		server  = firetest.New()
	)
	server.Start()
	defer server.Close()

	fb := New(server.URL, nil)
	err := fb.Update(payload)
	assert.NoError(t, err)

	v := server.Get("")
	assert.Equal(t, payload, v)
}
