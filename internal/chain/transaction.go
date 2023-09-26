package chain

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strings"
	"sync/atomic"

	"github.com/chainflag/eth-faucet/internal/chain/token"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
)

type TxBuilder interface {
	Sender() common.Address
	Transfer(ctx context.Context, to string, value *big.Int) (common.Hash, error)
	TransferTokens(ctx context.Context, to string, value *big.Int) (common.Hash, error)
}

type TxBuild struct {
	client       bind.ContractTransactor
	privateKey   *ecdsa.PrivateKey
	transactOpts *bind.TransactOpts
	signer       types.Signer
	fromAddress  common.Address
	tokenAddress common.Address
	nonce        uint64
}

func NewTxBuilder(provider string, privateKey *ecdsa.PrivateKey, chainID *big.Int, tokenAddress common.Address) (TxBuilder, error) {
	client, err := ethclient.Dial(provider)
	if err != nil {
		return nil, err
	}

	if chainID == nil {
		chainID, err = client.ChainID(context.Background())
		if err != nil {
			return nil, err
		}
	}

	transactOpts, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return nil, err
	}

	txBuilder := &TxBuild{
		client:       client,
		privateKey:   privateKey,
		signer:       types.NewEIP155Signer(chainID),
		transactOpts: transactOpts,
		fromAddress:  crypto.PubkeyToAddress(privateKey.PublicKey),
		tokenAddress: tokenAddress,
	}
	txBuilder.refreshNonce(context.Background())

	return txBuilder, nil
}

func (b *TxBuild) Sender() common.Address {
	return b.fromAddress
}

func (b *TxBuild) Transfer(ctx context.Context, to string, value *big.Int) (common.Hash, error) {
	gasLimit := uint64(21000)
	gasPrice, err := b.client.SuggestGasPrice(ctx)
	if err != nil {
		return common.Hash{}, err
	}

	toAddress := common.HexToAddress(to)
	unsignedTx := types.NewTx(&types.LegacyTx{
		Nonce:    b.getAndIncrementNonce(),
		To:       &toAddress,
		Value:    value,
		Gas:      gasLimit,
		GasPrice: gasPrice,
	})

	signedTx, err := types.SignTx(unsignedTx, b.signer, b.privateKey)
	if err != nil {
		return common.Hash{}, err
	}

	if err = b.client.SendTransaction(ctx, signedTx); err != nil {
		log.Error("failed to send tx", "tx hash", signedTx.Hash().String(), "err", err)
		if strings.Contains(err.Error(), "nonce") {
			b.refreshNonce(context.Background())
		}
		return common.Hash{}, err
	}

	return signedTx.Hash(), nil
}

func (b *TxBuild) TransferTokens(ctx context.Context, to string, value *big.Int) (common.Hash, error) {
	connectedToken, err := token.NewTokenTransactor(b.tokenAddress, b.client)
	if err != nil {
		return common.Hash{}, err
	}

	b.getAndIncrementNonce()
	tx, err := connectedToken.Transfer(b.transactOpts, common.HexToAddress(to), value)
	if err != nil {
		return common.Hash{}, err
	}

	return tx.Hash(), nil
}

func (b *TxBuild) getAndIncrementNonce() uint64 {
	return atomic.AddUint64(&b.nonce, 1) - 1
}

func (b *TxBuild) refreshNonce(ctx context.Context) {
	nonce, err := b.client.PendingNonceAt(ctx, b.Sender())
	if err != nil {
		log.Error("failed to refresh nonce", "address", b.Sender(), "err", err)
		return
	}

	b.nonce = nonce
}
