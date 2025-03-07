package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/getAlby/nostr-wallet-connect/alby"
	"github.com/getAlby/nostr-wallet-connect/api"
	"github.com/getAlby/nostr-wallet-connect/db"
	"github.com/getAlby/nostr-wallet-connect/lsp"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type WailsRequestRouterResponse struct {
	Body  interface{} `json:"body"`
	Error string      `json:"error"`
}

// TODO: make this match echo
func (app *WailsApp) WailsRequestRouter(route string, method string, body string) WailsRequestRouterResponse {
	ctx := app.ctx

	// the grouping is done to avoid other parameters like &unused=true
	albyCallbackRegex := regexp.MustCompile(
		`/api/alby/callback\?code=([^&]+)(&.*)?`,
	)

	authCodeMatch := albyCallbackRegex.FindStringSubmatch(route)

	switch {
	case len(authCodeMatch) > 1:
		code := authCodeMatch[1]

		err := app.svc.albyOAuthSvc.CallbackHandler(ctx, code)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to decode request to wails router")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: nil, Error: ""}
	}

	appRegex := regexp.MustCompile(
		`/api/apps/([0-9a-f]+)`,
	)

	appMatch := appRegex.FindStringSubmatch(route)

	switch {
	case len(appMatch) > 1:
		pubkey := appMatch[1]

		userApp := db.App{}
		findResult := app.svc.db.Where("nostr_pubkey = ?", pubkey).First(&userApp)

		if findResult.RowsAffected == 0 {
			return WailsRequestRouterResponse{Body: nil, Error: "App does not exist"}
		}

		switch method {
		case "GET":
			app := app.api.GetApp(&userApp)
			return WailsRequestRouterResponse{Body: app, Error: ""}
		case "PATCH":
			updateAppRequest := &api.UpdateAppRequest{}
			err := json.Unmarshal([]byte(body), updateAppRequest)
			if err != nil {
				app.svc.logger.WithFields(logrus.Fields{
					"route":  route,
					"method": method,
					"body":   body,
				}).WithError(err).Error("Failed to decode request to wails router")
				return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
			}
			err = app.api.UpdateApp(&userApp, updateAppRequest)
			if err != nil {
				return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
			}
			return WailsRequestRouterResponse{Body: nil, Error: ""}
		case "DELETE":
			err := app.api.DeleteApp(&userApp)
			if err != nil {
				return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
			}
			return WailsRequestRouterResponse{Body: nil, Error: ""}
		}
	}

	peerChannelRegex := regexp.MustCompile(
		`/api/peers/([^/]+)/channels/([^/]+)\?force=(.+)`,
	)

	peerChannelMatch := peerChannelRegex.FindStringSubmatch(route)

	switch {
	case len(peerChannelMatch) == 4:
		peerId := peerChannelMatch[1]
		channelId := peerChannelMatch[2]
		force := peerChannelMatch[3]
		switch method {
		case "DELETE":
			closeChannelResponse, err := app.api.CloseChannel(ctx, peerId, channelId, force == "true")
			if err != nil {
				return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
			}
			return WailsRequestRouterResponse{Body: closeChannelResponse, Error: ""}
		}
	}

	peerRegex := regexp.MustCompile(
		`/api/peers/([^/]+)`,
	)

	peerMatch := peerRegex.FindStringSubmatch(route)

	switch {
	case len(peerMatch) == 2:
		peerId := peerMatch[1]
		switch method {
		case "DELETE":
			err := app.api.DisconnectPeer(ctx, peerId)
			if err != nil {
				return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
			}
			return WailsRequestRouterResponse{Body: nil, Error: ""}
		}
	}

	networkGraphRegex := regexp.MustCompile(
		`/api/node/network-graph\?nodeIds=(.+)`,
	)

	networkGraphMatch := networkGraphRegex.FindStringSubmatch(route)

	switch {
	case len(networkGraphMatch) == 2:
		nodeIds := networkGraphMatch[1]
		networkGraphResponse, err := app.api.GetNetworkGraph(strings.Split(nodeIds, ","))
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: networkGraphResponse, Error: ""}
	}

	mempoolApiRegex := regexp.MustCompile(
		`/api/mempool\?endpoint=(.+)`,
	)
	mempoolApiEndpointMatch := mempoolApiRegex.FindStringSubmatch(route)

	switch {
	case len(mempoolApiEndpointMatch) > 1:
		endpoint := mempoolApiEndpointMatch[1]
		node, err := app.api.RequestMempoolApi(endpoint)
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}

		return WailsRequestRouterResponse{Body: node, Error: ""}
	}

	switch route {
	case "/api/alby/me":
		me, err := app.svc.albyOAuthSvc.GetMe(ctx)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to decode request to wails router")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: me, Error: ""}
	case "/api/alby/balance":
		balance, err := app.svc.albyOAuthSvc.GetBalance(ctx)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to decode request to wails router")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: &alby.AlbyBalanceResponse{
			Sats: balance.Balance,
		}, Error: ""}
	case "/api/alby/pay":
		payRequest := &alby.AlbyPayRequest{}
		err := json.Unmarshal([]byte(body), payRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to decode request to wails router")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		err = app.svc.albyOAuthSvc.SendPayment(ctx, payRequest.Invoice)
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: nil, Error: ""}
	case "/api/apps":
		switch method {
		case "GET":
			apps, err := app.api.ListApps()
			if err != nil {
				return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
			}
			return WailsRequestRouterResponse{Body: apps, Error: ""}
		case "POST":
			createAppRequest := &api.CreateAppRequest{}
			err := json.Unmarshal([]byte(body), createAppRequest)
			if err != nil {
				app.svc.logger.WithFields(logrus.Fields{
					"route":  route,
					"method": method,
					"body":   body,
				}).WithError(err).Error("Failed to decode request to wails router")
				return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
			}
			createAppResponse, err := app.api.CreateApp(createAppRequest)
			if err != nil {
				return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
			}
			return WailsRequestRouterResponse{Body: createAppResponse, Error: ""}
		}
	case "/api/reset-router":
		resetRouterRequest := &api.ResetRouterRequest{}
		err := json.Unmarshal([]byte(body), resetRouterRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to decode request to wails router")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}

		err = app.api.ResetRouter(resetRouterRequest.Key)
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		res := WailsRequestRouterResponse{Body: nil, Error: ""}
		return res
	case "/api/stop":
		err := app.api.Stop()
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		res := WailsRequestRouterResponse{Body: nil, Error: ""}
		return res
	case "/api/channels":
		switch method {
		case "GET":
			channels, err := app.api.ListChannels(ctx)
			if err != nil {
				return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
			}
			res := WailsRequestRouterResponse{Body: channels, Error: ""}
			return res
		case "POST":
			openChannelRequest := &api.OpenChannelRequest{}
			err := json.Unmarshal([]byte(body), openChannelRequest)
			if err != nil {
				app.svc.logger.WithFields(logrus.Fields{
					"route":  route,
					"method": method,
					"body":   body,
				}).WithError(err).Error("Failed to decode request to wails router")
				return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
			}
			openChannelResponse, err := app.api.OpenChannel(ctx, openChannelRequest)
			if err != nil {
				return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
			}
			return WailsRequestRouterResponse{Body: openChannelResponse, Error: ""}
		}
	case "/api/channels/suggestions":
		suggestions, err := app.api.GetChannelPeerSuggestions(ctx)
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		res := WailsRequestRouterResponse{Body: suggestions, Error: ""}
		return res
	case "/api/balances":
		balancesResponse, err := app.api.GetBalances(ctx)
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		res := WailsRequestRouterResponse{Body: *balancesResponse, Error: ""}
		return res
	case "/api/wallet/sync":
		app.api.SyncWallet()
		return WailsRequestRouterResponse{Body: nil, Error: ""}
	case "/api/wallet/address":
		address, err := app.api.GetUnusedOnchainAddress(ctx)
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: address, Error: ""}
	case "/api/wallet/new-address":
		newAddress, err := app.api.GetNewOnchainAddress(ctx)
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: newAddress, Error: ""}
	case "/api/wallet/redeem-onchain-funds":

		redeemOnchainFundsRequest := &api.RedeemOnchainFundsRequest{}
		err := json.Unmarshal([]byte(body), redeemOnchainFundsRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to decode request to wails router")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}

		redeemOnchainFundsResponse, err := app.api.RedeemOnchainFunds(ctx, redeemOnchainFundsRequest.ToAddress)
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: *redeemOnchainFundsResponse, Error: ""}
	case "/api/wallet/sign-message":
		signMessageRequest := &api.SignMessageRequest{}
		err := json.Unmarshal([]byte(body), signMessageRequest)
		signMessageResponse, err := app.api.SignMessage(ctx, signMessageRequest.Message)
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: *signMessageResponse, Error: ""}
	// TODO: review naming
	case "/api/instant-channel-invoices":
		newInstantChannelRequest := &lsp.NewInstantChannelInvoiceRequest{}
		err := json.Unmarshal([]byte(body), newInstantChannelRequest)
		newInstantChannelResponseResponse, err := app.api.GetLSPService().NewInstantChannelInvoice(ctx, newInstantChannelRequest)
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: *newInstantChannelResponseResponse, Error: ""}
	case "/api/peers":
		switch method {
		case "GET":
			peers, err := app.api.ListPeers(ctx)
			if err != nil {
				return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
			}
			return WailsRequestRouterResponse{Body: peers, Error: ""}
		case "POST":
			connectPeerRequest := &api.ConnectPeerRequest{}
			err := json.Unmarshal([]byte(body), connectPeerRequest)
			if err != nil {
				app.svc.logger.WithFields(logrus.Fields{
					"route":  route,
					"method": method,
					"body":   body,
				}).WithError(err).Error("Failed to decode request to wails router")
				return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
			}
			err = app.api.ConnectPeer(ctx, connectPeerRequest)
			if err != nil {
				return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
			}
			return WailsRequestRouterResponse{Body: nil, Error: ""}
		}
	case "/api/node/connection-info":
		nodeConnectionInfo, err := app.api.GetNodeConnectionInfo(ctx)
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: *nodeConnectionInfo, Error: ""}
	case "/api/node/status":
		nodeStatus, err := app.api.GetNodeStatus(ctx)
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: *nodeStatus, Error: ""}
	case "/api/info":
		infoResponse, err := app.api.GetInfo(ctx)
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		infoResponse.Unlocked = infoResponse.Running
		res := WailsRequestRouterResponse{Body: *infoResponse, Error: ""}
		return res
	case "/api/alby/link-account":
		err := app.svc.albyOAuthSvc.LinkAccount(ctx)
		if err != nil {
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		res := WailsRequestRouterResponse{Error: ""}
		return res
	case "/api/encrypted-mnemonic":
		infoResponse := app.api.GetEncryptedMnemonic()
		res := WailsRequestRouterResponse{Body: *infoResponse, Error: ""}
		return res
	case "/api/backup-reminder":
		backupReminderRequest := &api.BackupReminderRequest{}
		err := json.Unmarshal([]byte(body), backupReminderRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to decode request to wails router")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}

		err = app.api.SetNextBackupReminder(backupReminderRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to store backup reminder")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: nil, Error: ""}
	case "/api/unlock-password":
		changeUnlockPasswordRequest := &api.ChangeUnlockPasswordRequest{}
		err := json.Unmarshal([]byte(body), changeUnlockPasswordRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to decode request to wails router")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}

		err = app.api.ChangeUnlockPassword(changeUnlockPasswordRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to change unlock password")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: nil, Error: ""}
	case "/api/start":
		startRequest := &api.StartRequest{}
		err := json.Unmarshal([]byte(body), startRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to decode request to wails router")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		err = app.api.Start(startRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to setup node")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: nil, Error: ""}
	case "/api/csrf":
		return WailsRequestRouterResponse{Body: "dummy", Error: ""}
	case "/api/setup":
		setupRequest := &api.SetupRequest{}
		err := json.Unmarshal([]byte(body), setupRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to decode request to wails router")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		err = app.api.Setup(ctx, setupRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to setup node")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: nil, Error: ""}
	case "/api/send-payment-probes":
		sendPaymentProbesRequest := &api.SendPaymentProbesRequest{}
		err := json.Unmarshal([]byte(body), sendPaymentProbesRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to decode request to wails router")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		sendPaymentProbesResponse, err := app.api.SendPaymentProbes(ctx, sendPaymentProbesRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to send payment probes")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: sendPaymentProbesResponse, Error: ""}
	case "/api/send-spontaneous-payment-probes":
		sendSpontaneousPaymentProbesRequest := &api.SendSpontaneousPaymentProbesRequest{}
		err := json.Unmarshal([]byte(body), sendSpontaneousPaymentProbesRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to decode request to wails router")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		sendSpontaneousPaymentProbesResponse, err := app.api.SendSpontaneousPaymentProbes(ctx, sendSpontaneousPaymentProbesRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to send spontaneous payment probes")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: sendSpontaneousPaymentProbesResponse, Error: ""}
	case "/api/backup":
		backupRequest := &api.BasicBackupRequest{}
		err := json.Unmarshal([]byte(body), backupRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to decode request to wails router")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}

		saveFilePath, err := runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{
			Title:           "Save Backup File",
			DefaultFilename: "nwc.bkp",
		})
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to open save file dialog")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}

		backupFile, err := os.Create(saveFilePath)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to create backup file")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}

		defer backupFile.Close()

		err = app.api.GetBackupService().CreateBackup(backupRequest.UnlockPassword, backupFile)

		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to create backup")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: nil, Error: ""}
	case "/api/restore":
		restoreRequest := &api.BasicRestoreWailsRequest{}
		err := json.Unmarshal([]byte(body), restoreRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to decode request to wails router")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}

		backupFilePath, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
			Title:           "Select Backup File",
			DefaultFilename: "nwc.bkp",
		})
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to open save file dialog")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}

		backupFile, err := os.Open(backupFilePath)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to open backup file")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}

		defer backupFile.Close()

		err = app.api.GetBackupService().RestoreBackup(restoreRequest.UnlockPassword, backupFile)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to restore backup")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: nil, Error: ""}
	}

	if strings.HasPrefix(route, "/api/log/") {
		logType := strings.TrimPrefix(route, "/api/log/")
		if logType != api.LogTypeNode && logType != api.LogTypeApp {
			return WailsRequestRouterResponse{Body: nil, Error: fmt.Sprintf("Invalid log type: '%s'", logType)}
		}
		getLogOutputRequest := &api.GetLogOutputRequest{}
		err := json.Unmarshal([]byte(body), getLogOutputRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to decode request to wails router")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		logOutputResponse, err := app.api.GetLogOutput(ctx, logType, getLogOutputRequest)
		if err != nil {
			app.svc.logger.WithFields(logrus.Fields{
				"route":  route,
				"method": method,
				"body":   body,
			}).WithError(err).Error("Failed to get log output")
			return WailsRequestRouterResponse{Body: nil, Error: err.Error()}
		}
		return WailsRequestRouterResponse{Body: logOutputResponse, Error: ""}
	}

	app.svc.logger.WithFields(logrus.Fields{
		"route":  route,
		"method": method,
	}).Error("Unhandled route")
	return WailsRequestRouterResponse{Body: nil, Error: fmt.Sprintf("Unhandled route: %s %s", method, route)}
}
