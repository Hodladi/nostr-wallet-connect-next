package cashu

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/elnosh/gonuts/wallet"
	"github.com/elnosh/gonuts/wallet/storage"
	"github.com/getAlby/nostr-wallet-connect/lnclient"
	"github.com/getAlby/nostr-wallet-connect/nip47"
	decodepay "github.com/nbd-wtf/ln-decodepay"
	"github.com/sirupsen/logrus"
)

type CashuService struct {
	logger *logrus.Logger
	wallet *wallet.Wallet
}

func NewCashuService(logger *logrus.Logger, workDir string, mintUrl string) (result lnclient.LNClient, err error) {
	if workDir == "" {
		return nil, errors.New("one or more required cashu configuration are missing")
	}
	if mintUrl == "" {
		mintUrl = "https://8333.space:3338"
	}

	//create dir if not exists
	newpath := filepath.Join(workDir)
	err = os.MkdirAll(newpath, os.ModePerm)
	if err != nil {
		log.Printf("Failed to create cashu working dir: %v", err)
		return nil, err
	}

	logger.WithField("mintUrl", mintUrl).Info("Setting up cashu wallet")
	config := wallet.Config{WalletPath: newpath, CurrentMintURL: mintUrl}

	wallet, err := wallet.LoadWallet(config)
	if err != nil {
		logger.WithError(err).Error("Failed to load cashu wallet")
		return nil, err
	}

	cs := CashuService{
		logger: logger,
		wallet: wallet,
	}

	return &cs, nil
}

func (cs *CashuService) Shutdown() error {
	return nil
}

func (cs *CashuService) SendPaymentSync(ctx context.Context, invoice string) (response *lnclient.PayInvoiceResponse, err error) {
	meltResponse, err := cs.wallet.Melt(invoice, cs.wallet.CurrentMint())
	if err != nil {
		cs.logger.WithError(err).Error("Failed to melt invoice")
		return nil, err
	}

	if meltResponse == nil || meltResponse.Preimage == "" {
		return nil, errors.New("no preimage in melt response")
	}

	// TODO: get fee from melt response
	cashuInvoice, err := cs.wallet.GetInvoiceByPaymentRequest(invoice)
	if err != nil {
		cs.logger.WithField("invoice", invoice).WithError(err).Error("Failed to get invoice after melting")
		return nil, err
	}

	transaction, err := cs.cashuInvoiceToTransaction(cashuInvoice)
	if err != nil {
		cs.logger.WithField("invoice", invoice).WithError(err).Error("Failed to convert invoice to transaction")
		return nil, err
	}

	fee := uint64(transaction.FeesPaid)

	return &lnclient.PayInvoiceResponse{
		Preimage: meltResponse.Preimage,
		Fee:      &fee,
	}, nil
}

func (cs *CashuService) SendKeysend(ctx context.Context, amount int64, destination, preimage string, custom_records []lnclient.TLVRecord) (preImage string, err error) {
	return "", errors.New("Keysend not supported")
}

func (cs *CashuService) GetBalance(ctx context.Context) (balance int64, err error) {
	balanceByMints := cs.wallet.GetBalanceByMints()
	totalBalance := uint64(0)

	for _, balance := range balanceByMints {
		totalBalance += balance
	}

	return int64(totalBalance * 1000), nil
}

func (cs *CashuService) MakeInvoice(ctx context.Context, amount int64, description string, descriptionHash string, expiry int64) (transaction *nip47.Transaction, err error) {
	mintResponse, err := cs.wallet.RequestMint(uint64(amount / 1000))
	if err != nil {
		cs.logger.WithError(err).Error("Failed to mint")
		return nil, err
	}

	paymentRequest, err := decodepay.Decodepay(mintResponse.Request)
	if err != nil {
		cs.logger.WithFields(logrus.Fields{
			"invoice": mintResponse.Request,
		}).WithError(err).Error("Failed to decode bolt11 invoice")
		return nil, err
	}

	return cs.LookupInvoice(ctx, paymentRequest.PaymentHash)
}

func (cs *CashuService) LookupInvoice(ctx context.Context, paymentHash string) (transaction *nip47.Transaction, err error) {
	cashuInvoice := cs.wallet.GetInvoiceByPaymentHash(paymentHash)

	if cashuInvoice == nil {
		cs.logger.WithField("paymentHash", paymentHash).Error("Failed to lookup payment request by payment hash")
		return nil, errors.New("failed to lookup payment request by payment hash")
	}

	cs.checkInvoice(cashuInvoice)

	transaction, err = cs.cashuInvoiceToTransaction(cashuInvoice)

	return transaction, nil
}

func (cs *CashuService) ListTransactions(ctx context.Context, from, until, limit, offset uint64, unpaid bool, invoiceType string) (transactions []nip47.Transaction, err error) {
	transactions = []nip47.Transaction{}

	invoices := cs.wallet.GetAllInvoices()

	for _, invoice := range invoices {
		invoiceCreated := time.UnixMilli(invoice.CreatedAt * 1000)

		if time.Since(invoiceCreated) < 24*time.Hour {
			cs.checkInvoice(&invoice)
		}

		transaction, err := cs.cashuInvoiceToTransaction(&invoice)
		if err != nil {
			continue
		}
		if transaction.SettledAt == nil {
			continue
		}
		transactions = append(transactions, *transaction)
	}

	// sort by created date descending
	sort.SliceStable(transactions, func(i, j int) bool {
		return transactions[i].CreatedAt > transactions[j].CreatedAt
	})

	return transactions, nil
}

func (cs *CashuService) GetInfo(ctx context.Context) (info *lnclient.NodeInfo, err error) {
	return &lnclient.NodeInfo{
		Alias:       "NWC (Cashu)",
		Color:       "#897FFF",
		Pubkey:      "",
		Network:     "bitcoin",
		BlockHeight: 0,
		BlockHash:   "",
	}, nil
}

func (cs *CashuService) ListChannels(ctx context.Context) ([]lnclient.Channel, error) {
	return nil, nil
}

func (cs *CashuService) GetNodeConnectionInfo(ctx context.Context) (nodeConnectionInfo *lnclient.NodeConnectionInfo, err error) {
	return &lnclient.NodeConnectionInfo{}, nil
}

func (cs *CashuService) ConnectPeer(ctx context.Context, connectPeerRequest *lnclient.ConnectPeerRequest) error {
	return nil
}

func (cs *CashuService) OpenChannel(ctx context.Context, openChannelRequest *lnclient.OpenChannelRequest) (*lnclient.OpenChannelResponse, error) {
	return nil, nil
}

func (cs *CashuService) CloseChannel(ctx context.Context, closeChannelRequest *lnclient.CloseChannelRequest) (*lnclient.CloseChannelResponse, error) {
	return nil, nil
}

func (cs *CashuService) GetNewOnchainAddress(ctx context.Context) (string, error) {
	return "", nil
}

func (cs *CashuService) GetOnchainBalance(ctx context.Context) (*lnclient.OnchainBalanceResponse, error) {
	return &lnclient.OnchainBalanceResponse{
		Spendable: 0,
		Total:     0,
	}, nil
}

func (cs *CashuService) RedeemOnchainFunds(ctx context.Context, toAddress string) (string, error) {
	return "", nil
}

func (cs *CashuService) ResetRouter(key string) error {
	return nil
}

func (cs *CashuService) SignMessage(ctx context.Context, message string) (string, error) {
	return "", nil
}

func (cs *CashuService) DisconnectPeer(ctx context.Context, peerId string) error {
	return nil
}

func (cs *CashuService) ListPeers(ctx context.Context) ([]lnclient.PeerDetails, error) {
	return nil, nil
}
func (cs *CashuService) GetLogOutput(ctx context.Context, maxLen int) ([]byte, error) {
	return nil, nil
}

func (cs *CashuService) GetStorageDir() (string, error) {
	return "", nil
}
func (cs *CashuService) GetNetworkGraph(nodeIds []string) (lnclient.NetworkGraphResponse, error) {
	return nil, nil
}
func (cs *CashuService) UpdateLastWalletSyncRequest() {}

func (cs *CashuService) GetNodeStatus(ctx context.Context) (nodeStatus *lnclient.NodeStatus, err error) {
	return nil, nil
}

func (cs *CashuService) SendPaymentProbes(ctx context.Context, invoice string) error {
	return nil
}
func (cs *CashuService) SendSpontaneousPaymentProbes(ctx context.Context, amountMsat uint64, nodeId string) error {
	return nil
}

func (cs *CashuService) GetBalances(ctx context.Context) (*lnclient.BalancesResponse, error) {
	balance, err := cs.GetBalance(ctx)
	if err != nil {
		cs.logger.WithError(err).Error("Failed to get balance")
		return nil, err
	}
	return &lnclient.BalancesResponse{
		Onchain: lnclient.OnchainBalanceResponse{
			Spendable: 0,
			Total:     0,
		},
		Lightning: lnclient.LightningBalanceResponse{
			TotalSpendable:       balance,
			TotalReceivable:      0,
			NextMaxSpendable:     balance,
			NextMaxReceivable:    0,
			NextMaxSpendableMPP:  balance,
			NextMaxReceivableMPP: 0,
		},
	}, nil
}

func (cs *CashuService) cashuInvoiceToTransaction(cashuInvoice *storage.Invoice) (*nip47.Transaction, error) {
	paymentRequest, err := decodepay.Decodepay(cashuInvoice.PaymentRequest)
	if err != nil {
		cs.logger.WithFields(logrus.Fields{
			"invoice": cashuInvoice.PaymentRequest,
		}).WithError(err).Error("Failed to decode bolt11 invoice")
		return nil, err
	}

	var settledAt *int64
	if cashuInvoice.SettledAt > 0 {
		settledAt = &cashuInvoice.SettledAt
	}

	var expiresAt *int64

	expiresAtUnix := time.UnixMilli(int64(paymentRequest.CreatedAt) * 1000).Add(time.Duration(paymentRequest.Expiry) * time.Second).Unix()
	expiresAt = &expiresAtUnix
	description := paymentRequest.Description
	descriptionHash := paymentRequest.DescriptionHash

	invoiceType := "outgoing"
	if cashuInvoice.TransactionType == storage.Mint {
		invoiceType = "incoming"
	}

	return &nip47.Transaction{
		Type:            invoiceType,
		Invoice:         cashuInvoice.PaymentRequest,
		PaymentHash:     paymentRequest.PaymentHash,
		Amount:          paymentRequest.MSatoshi,
		CreatedAt:       int64(paymentRequest.CreatedAt),
		ExpiresAt:       expiresAt,
		Description:     description,
		DescriptionHash: descriptionHash,
		Preimage:        cashuInvoice.Preimage,
		SettledAt:       settledAt,
		FeesPaid:        int64(cashuInvoice.QuoteAmount*1000) - paymentRequest.MSatoshi,
	}, nil
}

func (cs *CashuService) checkInvoice(cashuInvoice *storage.Invoice) {
	if cashuInvoice.TransactionType == storage.Mint && !cashuInvoice.Paid {
		cs.logger.WithFields(logrus.Fields{
			"paymentHash": cashuInvoice.PaymentHash,
		}).Info("Checking unpaid invoice")

		proofs, err := cs.wallet.MintTokens(cashuInvoice.Id)
		if err != nil {
			cs.logger.WithFields(logrus.Fields{
				"paymentHash": cashuInvoice.PaymentHash,
			}).WithError(err).Warn("failed to mint")
		}

		if proofs != nil {
			cs.logger.WithFields(logrus.Fields{
				"paymentHash": cashuInvoice.PaymentHash,
				"amount":      proofs.Amount(),
			}).Info("sats successfully minted")
		}
	}
}
