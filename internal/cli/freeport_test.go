package cli

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFreeTCPPort_ReturnsUsablePort(t *testing.T) {
	port, err := FreeTCPPort()
	require.NoError(t, err)
	assert.Greater(t, port, 0)

	// The port must actually be bindable immediately afterward.
	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	require.NoError(t, err)
	defer l.Close()
}

func TestFreeTCPPort_ReturnsDistinctPortsAcrossCalls(t *testing.T) {
	p1, err := FreeTCPPort()
	require.NoError(t, err)
	p2, err := FreeTCPPort()
	require.NoError(t, err)
	assert.NotEqual(t, p1, p2, "sequential calls should not race onto the same still-released port in practice")
}
