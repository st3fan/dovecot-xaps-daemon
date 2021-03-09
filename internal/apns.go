package internal

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/freswa/dovecot-xaps-daemon/internal/config"
	"github.com/freswa/dovecot-xaps-daemon/internal/database"
	"github.com/freswa/dovecot-xaps-daemon/pkg/apple_xserver_certs"
	"github.com/sideshow/apns2"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/http2"
	"net/http"
	"sync"
	"time"
)

var (
	oidUid        = []int{0, 9, 2342, 19200300, 100, 1, 1}
	productionOID = []int{1, 2, 840, 113635, 100, 6, 3, 2}
	GeoTrustCert  = "-----BEGIN CERTIFICATE-----\nMIIDVDCCAjygAwIBAgIDAjRWMA0GCSqGSIb3DQEBBQUAMEIxCzAJBgNVBAYTAlVT\nMRYwFAYDVQQKEw1HZW9UcnVzdCBJbmMuMRswGQYDVQQDExJHZW9UcnVzdCBHbG9i\nYWwgQ0EwHhcNMDIwNTIxMDQwMDAwWhcNMjIwNTIxMDQwMDAwWjBCMQswCQYDVQQG\nEwJVUzEWMBQGA1UEChMNR2VvVHJ1c3QgSW5jLjEbMBkGA1UEAxMSR2VvVHJ1c3Qg\nR2xvYmFsIENBMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA2swYYzD9\n9BcjGlZ+W988bDjkcbd4kdS8odhM+KhDtgPpTSEHCIjaWC9mOSm9BXiLnTjoBbdq\nfnGk5sRgprDvgOSJKA+eJdbtg/OtppHHmMlCGDUUna2YRpIuT8rxh0PBFpVXLVDv\niS2Aelet8u5fa9IAjbkU+BQVNdnARqN7csiRv8lVK83Qlz6cJmTM386DGXHKTubU\n1XupGc1V3sjs0l44U+VcT4wt/lAjNvxm5suOpDkZALeVAjmRCw7+OC7RHQWa9k0+\nbw8HHa8sHo9gOeL6NlMTOdReJivbPagUvTLrGAMoUgRx5aszPeE4uwc2hGKceeoW\nMPRfwCvocWvk+QIDAQABo1MwUTAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBTA\nephojYn7qwVkDBF9qn1luMrMTjAfBgNVHSMEGDAWgBTAephojYn7qwVkDBF9qn1l\nuMrMTjANBgkqhkiG9w0BAQUFAAOCAQEANeMpauUvXVSOKVCUn5kaFOSPeCpilKIn\nZ57QzxpeR+nBsqTP3UEaBU6bS+5Kb1VSsyShNwrrZHYqLizz/Tt1kL/6cdjHPTfS\ntQWVYrmm3ok9Nns4d0iXrKYgjy6myQzCsplFAMfOEVEiIuCl6rYVSAlk6l5PdPcF\nPseKUgzbFbS9bZvlxrFUaKnjaZC2mqUPuLk/IH2uSrW4nOQdtqvmlKXBx4Ot2/Un\nhw4EbNX/3aBd7YdStysVAq45pmp06drE57xNNB6pXE0zX5IJL4hmXXeXxx12E6nV\n5fEWCRE11azbJHFwLJhWC9kXtNHjUStedejV0NxPNO3CBWaAocvmMw==\n-----END CERTIFICATE-----"
)

type Apns struct {
	DelayTime            uint
	Topic                string
	CheckDelayedInterval uint
	client               *apns2.Client
	db                   *database.Database
	mapMutex             sync.Mutex
	delayedApns          map[database.Registration]time.Time
}

func NewApns(config *config.Config, db *database.Database) (apns *Apns) {
	apns = &Apns{
		DelayTime:            config.Delay,
		CheckDelayedInterval: config.CheckInterval,
		db:                   db,
		mapMutex:             sync.Mutex{},
		delayedApns:          make(map[database.Registration]time.Time),
	}
	log.Debugln("APNS for non NewMessage events will be delayed for", time.Second*time.Duration(apns.DelayTime))
	log.Debugln("Trying to get existing certs from the DB")
	// try to retrieve certs from the db
	certs, successful := apns.db.GetCerts()
	// if we got some certs but they are no longer than 30 days valid
	if successful && invalidAfterFromCertificate(certs.Mail) < time.Hour*24*30 {
		certs = apple_xserver_certs.RenewCerts(certs, config.AppleId, config.AppleIdHashedPassword)
		apns.db.PutCerts(certs)
	}
	if !successful {
		certs = apple_xserver_certs.NewCerts(config.AppleId, config.AppleIdHashedPassword)
		apns.db.PutCerts(certs)
	}
	// extract the mail cert and retrieve its topic
	mailCert := certs.Mail
	topic, err := topicFromCertificate(*mailCert)

	if err != nil {
		log.Fatalln("Could not parse apns topic from certificate: ", err)
	}
	apns.Topic = topic
	log.Debugln("Topic is", apns.Topic)
	apns.client = apns2.NewClient(*mailCert).Production()

	// Get the SystemCertPool, continue with an empty pool on error
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	// Append our cert to the system pool
	if ok := rootCAs.AppendCertsFromPEM([]byte(GeoTrustCert)); !ok {
		log.Infoln("No certs appended, using system certs only")
	}
	apns.client.HTTPClient.Transport.(*http2.Transport).TLSClientConfig.RootCAs = rootCAs

	apns.createDelayedNotificationThread()
	return apns
}

func (apns *Apns) createDelayedNotificationThread() {
	delayedNotificationTicker := time.NewTicker(time.Second * time.Duration(apns.CheckDelayedInterval))
	go func() {
		for range delayedNotificationTicker.C {
			apns.checkDelayed()
		}
	}()
}

func (apns *Apns) checkDelayed() {
	log.Debugln("Checking all delayed APNS")
	var sendNow []database.Registration
	apns.mapMutex.Lock()
	for reg, t := range apns.delayedApns {
		log.Debugln("Registration", reg.AccountId, "/", reg.DeviceToken, "has been waiting for", time.Since(t))
		if time.Since(t) > time.Second*time.Duration(apns.DelayTime) {
			sendNow = append(sendNow, reg)
			delete(apns.delayedApns, reg)
		}
	}
	apns.mapMutex.Unlock()
	for _, reg := range sendNow {
		apns.SendNotification(reg, false)
	}
}

func (apns *Apns) SendNotification(registration database.Registration, delayed bool) {
	apns.mapMutex.Lock()
	if delayed {
		apns.delayedApns[registration] = time.Now()
		apns.mapMutex.Unlock()
		return
	} else {
		delete(apns.delayedApns, registration)
		apns.mapMutex.Unlock()
	}
	log.Debugln("Sending notification to", registration.AccountId, "/", registration.DeviceToken)

	notification := &apns2.Notification{}
	notification.DeviceToken = registration.DeviceToken
	notification.Topic = apns.Topic
	composedPayload := []byte(`{"aps":{`)
	composedPayload = append(composedPayload, []byte(`"account-id":"`+registration.AccountId+`"`)...)
	composedPayload = append(composedPayload, []byte(`}}`)...)
	notification.Payload = composedPayload
	notification.Expiration = time.Now().Add(24 * time.Hour)
	// set the apns-priority
	//notification.Priority = apns2.PriorityLow

	if log.IsLevelEnabled(log.DebugLevel) {
		dbgstr, _ := notification.MarshalJSON()
		log.Debugf("Sending: %s", dbgstr)
	}
	res, err := apns.client.Push(notification)

	if err != nil {
		log.Fatal("Error:", err)
	}

	switch res.StatusCode {
	case http.StatusOK:
		log.Debugln("Apple returned 200 for notification to", registration.AccountId, "/", registration.DeviceToken)
	case 410:
		// The device token is inactive for the specified topic.
		log.Infoln("Apple returned 410 for notification to", registration.AccountId, "/", registration.DeviceToken)
		apns.db.DeleteIfExistRegistration(registration.DeviceToken)
	default:
		log.Errorf("Apple returned a non-200 HTTP status: %v %v %v\n", res.StatusCode, res.ApnsID, res.Reason)
	}
}

func topicFromCertificate(tlsCert tls.Certificate) (string, error) {
	if len(tlsCert.Certificate) > 1 {
		return "", errors.New("found multiple certificates in the cert file - only one is allowed")
	}

	cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		log.Fatalln("Could not parse certificate: ", err)
	}

	if len(cert.Subject.Names) == 0 {
		return "", errors.New("Subject.Names is empty")
	}

	if !cert.Subject.Names[0].Type.Equal(oidUid) {
		return "", errors.New("did not find a Subject.Names[0] with type 0.9.2342.19200300.100.1.1")
	}

	if !cert.Extensions[7].Id.Equal(productionOID) {
		return "", errors.New("did not find an Extensions[7] with Id 1.2.840.113635.100.6.3.2 " +
			"which would label this certificate for production use")
	}

	return cert.Subject.Names[0].Value.(string), nil
}

// returns the duration until the certificate is invalid
func invalidAfterFromCertificate(certificate *tls.Certificate) time.Duration {
	cert, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		log.Fatalln("Could not parse certificate: ", err)
	}
	return cert.NotAfter.Sub(time.Now())
}
