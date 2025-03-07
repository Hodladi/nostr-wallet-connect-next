// TODO: move to nip47
package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/getAlby/nostr-wallet-connect/db"
	"github.com/getAlby/nostr-wallet-connect/events"
	"github.com/getAlby/nostr-wallet-connect/nip47"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"github.com/sirupsen/logrus"
)

type Relay interface {
	Publish(ctx context.Context, event nostr.Event) error
}

type Nip47Notifier struct {
	svc   *Service
	relay Relay
}

func NewNip47Notifier(svc *Service, relay Relay) *Nip47Notifier {
	return &Nip47Notifier{
		svc:   svc,
		relay: relay,
	}
}

func (notifier *Nip47Notifier) ConsumeEvent(ctx context.Context, event *events.Event) error {
	if event.Event != "nwc_payment_received" {
		return nil
	}

	if notifier.svc.lnClient == nil {
		return nil
	}

	paymentReceivedEventProperties, ok := event.Properties.(*events.PaymentReceivedEventProperties)
	if !ok {
		notifier.svc.logger.WithField("event", event).Error("Failed to cast event")
		return errors.New("failed to cast event")
	}

	transaction, err := notifier.svc.lnClient.LookupInvoice(ctx, paymentReceivedEventProperties.PaymentHash)
	if err != nil {
		notifier.svc.logger.
			WithField("paymentHash", paymentReceivedEventProperties.PaymentHash).
			WithError(err).
			Error("Failed to lookup invoice by payment hash")
		return err
	}

	notifier.notifySubscribers(ctx, &nip47.Notification{
		Notification:     transaction,
		NotificationType: nip47.PAYMENT_RECEIVED_NOTIFICATION,
	}, nostr.Tags{})
	return nil
}

func (notifier *Nip47Notifier) notifySubscribers(ctx context.Context, notification *nip47.Notification, tags nostr.Tags) {
	apps := []db.App{}

	// TODO: join apps and permissions
	notifier.svc.db.Find(&apps)

	for _, app := range apps {
		hasPermission, _, _ := notifier.svc.hasPermission(&app, nip47.NOTIFICATIONS_PERMISSION, 0)
		if !hasPermission {
			continue
		}
		notifier.notifySubscriber(ctx, &app, notification, tags)
	}
}

func (notifier *Nip47Notifier) notifySubscriber(ctx context.Context, app *db.App, notification *nip47.Notification, tags nostr.Tags) {
	notifier.svc.logger.WithFields(logrus.Fields{
		"notification": notification,
		"appId":        app.ID,
	}).Info("Notifying subscriber")

	ss, err := nip04.ComputeSharedSecret(app.NostrPubkey, notifier.svc.cfg.GetNostrSecretKey())
	if err != nil {
		notifier.svc.logger.WithFields(logrus.Fields{
			"notification": notification,
			"appId":        app.ID,
		}).WithError(err).Error("Failed to compute shared secret")
		return
	}

	payloadBytes, err := json.Marshal(notification)
	if err != nil {
		notifier.svc.logger.WithFields(logrus.Fields{
			"notification": notification,
			"appId":        app.ID,
		}).WithError(err).Error("Failed to stringify notification")
		return
	}
	msg, err := nip04.Encrypt(string(payloadBytes), ss)
	if err != nil {
		notifier.svc.logger.WithFields(logrus.Fields{
			"notification": notification,
			"appId":        app.ID,
		}).WithError(err).Error("Failed to encrypt notification payload")
		return
	}

	allTags := nostr.Tags{[]string{"p", app.NostrPubkey}}
	allTags = append(allTags, tags...)

	event := &nostr.Event{
		PubKey:    notifier.svc.cfg.GetNostrPublicKey(),
		CreatedAt: nostr.Now(),
		Kind:      nip47.NOTIFICATION_KIND,
		Tags:      allTags,
		Content:   msg,
	}
	err = event.Sign(notifier.svc.cfg.GetNostrSecretKey())
	if err != nil {
		notifier.svc.logger.WithFields(logrus.Fields{
			"notification": notification,
			"appId":        app.ID,
		}).WithError(err).Error("Failed to sign event")
		return
	}

	err = notifier.relay.Publish(ctx, *event)
	if err != nil {
		notifier.svc.logger.WithFields(logrus.Fields{
			"notification": notification,
			"appId":        app.ID,
		}).WithError(err).Error("Failed to publish notification")
		return
	}
	notifier.svc.logger.WithFields(logrus.Fields{
		"notification": notification,
		"appId":        app.ID,
	}).Info("Published notification event")

}
