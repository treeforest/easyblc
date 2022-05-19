package walletmgr

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWalletManager(t *testing.T) {
	mgr := New()
	_, err := mgr.CreateWallet()
	require.NoError(t, err)

	for addr, wallet := range mgr.Wallets {
		t.Logf("addr:%v pubKeyHash160:%x\n", addr, wallet.PubKeyHash160())
	}
}
