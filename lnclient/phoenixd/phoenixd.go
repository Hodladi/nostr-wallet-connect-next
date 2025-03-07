package phoenixd

import (
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/getAlby/nostr-wallet-connect/lnclient"
	"github.com/getAlby/nostr-wallet-connect/nip47"

	"github.com/sirupsen/logrus"
)

type InvoiceResponse struct {
	PaymentHash string `json:"paymentHash"`
	Preimage    string `json:"preimage"`
	ExternalId  string `json:"externalId"`
	Description string `json:"description"`
	Invoice     string `json:"invoice"`
	IsPaid      bool   `json:"isPaid"`
	ReceivedSat int64  `json:"receivedSat"`
	Fees        int64  `json:"fees"`
	CompletedAt int64  `json:"completedAt"`
	CreatedAt   int64  `json:"createdAt"`
}

type OutgoingPaymentResponse struct {
	PaymentHash string `json:"paymentHash"`
	Preimage    string `json:"preimage"`
	Invoice     string `json:"invoice"`
	IsPaid      bool   `json:"isPaid"`
	Sent        int64  `json:"sent"`
	Fees        int64  `json:"fees"`
	CompletedAt int64  `json:"completedAt"`
	CreatedAt   int64  `json:"createdAt"`
}

type PayResponse struct {
	PaymentHash     string `json:"paymentHash"`
	PaymentId       string `json:"paymentId"`
	PaymentPreimage string `json:"paymentPreimage"`
	RoutingFeeSat   int64  `json:"routingFeeSat"`
}

type MakeInvoiceResponse struct {
	AmountSat   int64  `json:"amountSat"`
	PaymentHash string `json:"paymentHash"`
	Serialized  string `json:"serialized"`
}

type InfoResponse struct {
	NodeId string `json:"nodeId"`
}

type BalanceResponse struct {
	BalanceSat   int64 `json:"balanceSat"`
	FeeCreditSat int64 `json:"feeCreditSat"`
}

type PhoenixService struct {
	Address       string
	Authorization string
	Logger        *logrus.Logger
}

func NewPhoenixService(logger *logrus.Logger, address string, authorization string) (result lnclient.LNClient, err error) {
	authorizationBase64 := b64.StdEncoding.EncodeToString([]byte(":" + authorization))
	phoenixService := &PhoenixService{Logger: logger, Address: address, Authorization: authorizationBase64}

	return phoenixService, nil
}

func (svc *PhoenixService) GetBalance(ctx context.Context) (balance int64, err error) {
	req, err := http.NewRequest(http.MethodGet, svc.Address+"/getbalance", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Add("Authorization", "Basic "+svc.Authorization)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var balanceRes BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&balanceRes); err != nil {
		return 0, err
	}

	balance = balanceRes.BalanceSat + balanceRes.FeeCreditSat
	return balance * 1000, nil
}

func (svc *PhoenixService) GetBalances(ctx context.Context) (*lnclient.BalancesResponse, error) {
	balance, err := svc.GetBalance(ctx)
	if err != nil {
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

func (svc *PhoenixService) ListTransactions(ctx context.Context, from, until, limit, offset uint64, unpaid bool, invoiceType string) (transactions []nip47.Transaction, err error) {
	incomingQuery := url.Values{}
	if from != 0 {
		incomingQuery.Add("from", strconv.FormatUint(from*1000, 10))
	}
	if until != 0 {
		incomingQuery.Add("to", strconv.FormatUint(until*1000, 10))
	}
	if limit != 0 {
		incomingQuery.Add("limit", strconv.FormatUint(limit, 10))
	}
	if offset != 0 {
		incomingQuery.Add("offset", strconv.FormatUint(offset, 10))
	}
	incomingQuery.Add("all", strconv.FormatBool(unpaid))

	incomingUrl := svc.Address + "/payments/incoming?" + incomingQuery.Encode()

	svc.Logger.WithFields(logrus.Fields{
		"url": incomingUrl,
	}).Infof("Fetching incoming tranasctions: %s", incomingUrl)
	incomingReq, err := http.NewRequest(http.MethodGet, incomingUrl, nil)
	if err != nil {
		return nil, err
	}
	incomingReq.Header.Add("Authorization", "Basic "+svc.Authorization)
	client := &http.Client{Timeout: 5 * time.Second}

	incomingResp, err := client.Do(incomingReq)
	if err != nil {
		return nil, err
	}
	defer incomingResp.Body.Close()

	var incomingPayments []InvoiceResponse
	if err := json.NewDecoder(incomingResp.Body).Decode(&incomingPayments); err != nil {
		return nil, err
	}
	transactions = []nip47.Transaction{}
	for _, invoice := range incomingPayments {
		var settledAt *int64
		if invoice.CompletedAt != 0 {
			settledAtUnix := time.UnixMilli(invoice.CompletedAt).Unix()
			settledAt = &settledAtUnix
		}
		transaction := nip47.Transaction{
			Type:        "incoming",
			Invoice:     invoice.Invoice,
			Preimage:    invoice.Preimage,
			PaymentHash: invoice.PaymentHash,
			Amount:      invoice.ReceivedSat * 1000,
			FeesPaid:    invoice.Fees * 1000,
			CreatedAt:   time.UnixMilli(invoice.CreatedAt).Unix(),
			Description: invoice.Description,
			SettledAt:   settledAt,
		}
		transactions = append(transactions, transaction)
	}

	// get outgoing payments
	outgoingQuery := url.Values{}
	if from != 0 {
		outgoingQuery.Add("from", strconv.FormatUint(from*1000, 10))
	}
	if until != 0 {
		outgoingQuery.Add("to", strconv.FormatUint(until*1000, 10))
	}
	if limit != 0 {
		outgoingQuery.Add("limit", strconv.FormatUint(limit, 10))
	}
	if offset != 0 {
		outgoingQuery.Add("offset", strconv.FormatUint(offset, 10))
	}
	outgoingQuery.Add("all", strconv.FormatBool(unpaid))

	outgoingUrl := svc.Address + "/payments/outgoing?" + outgoingQuery.Encode()

	svc.Logger.WithFields(logrus.Fields{
		"url": outgoingUrl,
	}).Infof("Fetching outgoing tranasctions: %s", outgoingUrl)
	outgoingReq, err := http.NewRequest(http.MethodGet, outgoingUrl, nil)
	if err != nil {
		return nil, err
	}
	outgoingReq.Header.Add("Authorization", "Basic "+svc.Authorization)
	outgoingResp, err := client.Do(outgoingReq)
	if err != nil {
		return nil, err
	}
	defer outgoingResp.Body.Close()

	var outgoingPayments []OutgoingPaymentResponse
	if err := json.NewDecoder(outgoingResp.Body).Decode(&outgoingPayments); err != nil {
		return nil, err
	}
	for _, invoice := range outgoingPayments {
		var settledAt *int64
		if invoice.CompletedAt != 0 {
			settledAtUnix := time.UnixMilli(invoice.CompletedAt).Unix()
			settledAt = &settledAtUnix
		}
		transaction := nip47.Transaction{
			Type:        "outgoing",
			Invoice:     invoice.Invoice,
			Preimage:    invoice.Preimage,
			PaymentHash: invoice.PaymentHash,
			Amount:      invoice.Sent * 1000,
			FeesPaid:    invoice.Fees * 1000,
			CreatedAt:   time.UnixMilli(invoice.CreatedAt).Unix(),
			SettledAt:   settledAt,
		}
		transactions = append(transactions, transaction)
	}

	// sort by created date descending
	sort.SliceStable(transactions, func(i, j int) bool {
		return transactions[i].CreatedAt > transactions[j].CreatedAt
	})

	return transactions, nil
}

func (svc *PhoenixService) GetInfo(ctx context.Context) (info *lnclient.NodeInfo, err error) {
	req, err := http.NewRequest(http.MethodGet, svc.Address+"/getinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Basic "+svc.Authorization)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var infoRes InfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&infoRes); err != nil {
		return nil, err
	}
	return &lnclient.NodeInfo{
		Alias:       "Phoenix",
		Color:       "",
		Pubkey:      infoRes.NodeId,
		Network:     "bitcoin",
		BlockHeight: 0,
		BlockHash:   "",
	}, nil
}

func (svc *PhoenixService) ListChannels(ctx context.Context) ([]lnclient.Channel, error) {
	channels := []lnclient.Channel{}
	return channels, nil
}

func (svc *PhoenixService) MakeInvoice(ctx context.Context, amount int64, description string, descriptionHash string, expiry int64) (transaction *nip47.Transaction, err error) {
	form := url.Values{}
	amountSat := strconv.FormatInt(amount/1000, 10)
	form.Add("amountSat", amountSat)
	if descriptionHash != "" {
		form.Add("descriptionHash", descriptionHash)
	} else if description != "" {
		form.Add("description", description)
	} else {
		form.Add("description", "invoice")
	}

	today := time.Now().UTC().Format("2006-02-01") // querying is too slow so we limit the invoices we query with the date - see list transactions
	form.Add("externalId", today)                  // for some resone phoenixd requires an external id to query a list of invoices. thus we set this to nwc
	svc.Logger.WithFields(logrus.Fields{
		"externalId": today,
		"amountSat":  amountSat,
	}).Infof("Requesting phoenix invoice")
	req, err := http.NewRequest(http.MethodPost, svc.Address+"/createinvoice", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Basic "+svc.Authorization)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var invoiceRes MakeInvoiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&invoiceRes); err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(1 * time.Hour).Unix()

	tx := &nip47.Transaction{
		Type:        "incoming",
		Invoice:     invoiceRes.Serialized,
		Preimage:    "",
		PaymentHash: invoiceRes.PaymentHash,
		FeesPaid:    0,
		CreatedAt:   time.Now().Unix(),
		ExpiresAt:   &expiresAt,
		SettledAt:   nil,
		Metadata:    nil,
	}
	return tx, nil
}

func (svc *PhoenixService) LookupInvoice(ctx context.Context, paymentHash string) (transaction *nip47.Transaction, err error) {
	req, err := http.NewRequest(http.MethodGet, svc.Address+"/payments/incoming/"+paymentHash, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Basic "+svc.Authorization)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var invoiceRes InvoiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&invoiceRes); err != nil {
		return nil, err
	}

	var settledAt *int64
	if invoiceRes.CompletedAt != 0 {
		settledAtUnix := time.UnixMilli(invoiceRes.CompletedAt).Unix()
		settledAt = &settledAtUnix
	}
	transaction = &nip47.Transaction{
		Type:        "incoming",
		Invoice:     invoiceRes.Invoice,
		Preimage:    invoiceRes.Preimage,
		PaymentHash: invoiceRes.PaymentHash,
		Amount:      invoiceRes.ReceivedSat * 1000,
		FeesPaid:    invoiceRes.Fees * 1000,
		CreatedAt:   time.UnixMilli(invoiceRes.CreatedAt).Unix(),
		Description: invoiceRes.Description,
		SettledAt:   settledAt,
	}
	return transaction, nil
}

func (svc *PhoenixService) SendPaymentSync(ctx context.Context, payReq string) (*lnclient.PayInvoiceResponse, error) {
	form := url.Values{}
	form.Add("invoice", payReq)
	req, err := http.NewRequest(http.MethodPost, svc.Address+"/payinvoice", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Basic "+svc.Authorization)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payRes PayResponse
	if err := json.NewDecoder(resp.Body).Decode(&payRes); err != nil {
		return nil, err
	}

	fee := uint64(payRes.RoutingFeeSat) * 1000
	return &lnclient.PayInvoiceResponse{
		Preimage: payRes.PaymentPreimage,
		Fee:      &fee,
	}, nil
}

func (svc *PhoenixService) SendKeysend(ctx context.Context, amount int64, destination, preimage string, custom_records []lnclient.TLVRecord) (respPreimage string, err error) {
	return "", errors.New("not implemented")
}

func (svc *PhoenixService) RedeemOnchainFunds(ctx context.Context, toAddress string) (txId string, err error) {
	return "", errors.New("not implemented")
}

func (svc *PhoenixService) ResetRouter(key string) error {
	return nil
}

func (svc *PhoenixService) Shutdown() error {
	return nil
}

func (svc *PhoenixService) GetNodeConnectionInfo(ctx context.Context) (nodeConnectionInfo *lnclient.NodeConnectionInfo, err error) {
	req, err := http.NewRequest(http.MethodGet, svc.Address+"/getinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Basic "+svc.Authorization)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var infoRes InfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&infoRes); err != nil {
		return nil, err
	}
	return &lnclient.NodeConnectionInfo{
		Pubkey: infoRes.NodeId,
	}, nil
}

func (svc *PhoenixService) ConnectPeer(ctx context.Context, connectPeerRequest *lnclient.ConnectPeerRequest) error {
	return nil
}
func (svc *PhoenixService) OpenChannel(ctx context.Context, openChannelRequest *lnclient.OpenChannelRequest) (*lnclient.OpenChannelResponse, error) {
	return nil, nil
}

func (svc *PhoenixService) CloseChannel(ctx context.Context, closeChannelRequest *lnclient.CloseChannelRequest) (*lnclient.CloseChannelResponse, error) {
	return nil, nil
}

func (svc *PhoenixService) GetNewOnchainAddress(ctx context.Context) (string, error) {
	return "", nil
}

func (svc *PhoenixService) GetOnchainBalance(ctx context.Context) (*lnclient.OnchainBalanceResponse, error) {
	return nil, errors.New("not implemented")
}

func (svc *PhoenixService) SignMessage(ctx context.Context, message string) (string, error) {
	return "", errors.New("not implemented")
}

func (svc *PhoenixService) SendPaymentProbes(ctx context.Context, invoice string) error {
	return nil
}

func (svc *PhoenixService) SendSpontaneousPaymentProbes(ctx context.Context, amountMsat uint64, nodeId string) error {
	return nil
}

func (svc *PhoenixService) ListPeers(ctx context.Context) ([]lnclient.PeerDetails, error) {
	return nil, nil
}

func (svc *PhoenixService) GetLogOutput(ctx context.Context, maxLen int) ([]byte, error) {
	return []byte{}, nil
}

func (svc *PhoenixService) GetNodeStatus(ctx context.Context) (nodeStatus *lnclient.NodeStatus, err error) {
	return nil, nil
}

func (svc *PhoenixService) GetStorageDir() (string, error) {
	return "", nil
}

func (svc *PhoenixService) GetNetworkGraph(nodeIds []string) (lnclient.NetworkGraphResponse, error) {
	return nil, nil
}

func (svc *PhoenixService) UpdateLastWalletSyncRequest() {}
func (svc *PhoenixService) DisconnectPeer(ctx context.Context, peerId string) error {
	return nil
}
