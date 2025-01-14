// stm: ignore
// Only tests external library behavior, therefore it should not be annotated
package beacon

import (
	"os"
	"testing"

	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"

	dchain "github.com/drand/drand/chain"
	hclient "github.com/drand/drand/client/http"
	"github.com/stretchr/testify/assert"
)

func TestPrintGroupInfo(t *testing.T) {
	tf.UnitTest(t)
	server := config.DrandConfigs[config.DrandDevnet].Servers[0]
	c, err := hclient.New(server, nil, nil)
	assert.NoError(t, err)
	cg := c.(interface {
		FetchChainInfo(groupHash []byte) (*dchain.Info, error)
	})
	chain, err := cg.FetchChainInfo(nil)
	assert.NoError(t, err)
	err = chain.ToJSON(os.Stdout)
	assert.NoError(t, err)
}

func TestMaxBeaconRoundForEpoch(t *testing.T) {
	tf.UnitTest(t)
	todayTS := uint64(1652222222)
	drandCfg := config.DrandConfigs[config.DrandDevnet]
	db, err := NewDrandBeacon(todayTS, config.NewDefaultConfig().NetworkParams.BlockDelay, drandCfg)
	assert.NoError(t, err)
	mbr15 := db.MaxBeaconRoundForEpoch(network.Version15, 100)
	mbr16 := db.MaxBeaconRoundForEpoch(network.Version16, 100)
	assert.Equal(t, mbr15+1, mbr16)
}
