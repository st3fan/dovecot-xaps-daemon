package aps

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/freswa/dovecot-xaps-daemon/database"
	"github.com/sideshow/apns2"
	log "github.com/sirupsen/logrus"
	"net/http"
	"sync"
	"time"
)

var oidUid = []int{0, 9, 2342, 19200300, 100, 1, 1}
var productionOID = []int{1, 2, 840, 113635, 100, 6, 3, 2}

var client *apns2.Client
var topic string
var db *database.Database
var mapMutex = &sync.Mutex{}
var delayedApns = make(map[database.Registration]time.Time)
var delayTime uint = 30

func NewApns(certFile string, keyFile string, checkDelayedInterval uint, delayMessageTime uint, database *database.Database) string {
	log.Debugln("APNS for non NewMessage events will be delayed for", time.Second*time.Duration(delayTime))
	delayTime = delayMessageTime
	db = database
	log.Debugln("Parsing", certFile, "to obtain APNS Topic")
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatalln("Could not parse certificate: ", err)
	}
	topic, err = topicFromCertificate(cert)
	if err != nil {
		log.Fatalln("Could not parse apns topic from certificate: ", err)
	}
	log.Debugln("Topic is", topic)

	log.Debugln("Creating APNS client")
	client = apns2.NewClient(cert).Production()

	delayedNotificationTicker := time.NewTicker(time.Second * time.Duration(checkDelayedInterval))
	go func() {
		for range delayedNotificationTicker.C {
			checkDelayed()
		}
	}()

	return topic
}

func checkDelayed() {
	log.Debugln("Checking all delayed APNS")
	var sendNow []database.Registration
	mapMutex.Lock()
	for reg, t := range delayedApns {
		log.Debugln("Registration", reg.AccountId, "/", reg.DeviceToken, "has been waiting for", time.Since(t))
		if time.Since(t) > time.Second*time.Duration(delayTime) {
			sendNow = append(sendNow, reg)
			delete(delayedApns, reg)
		}
	}
	mapMutex.Unlock()
	for _, reg := range sendNow {
		SendNotification(reg, false)
	}
}

func SendNotification(registration database.Registration, delayed bool) {
	mapMutex.Lock()
	if delayed {
		delayedApns[registration] = time.Now()
		mapMutex.Unlock()
		return
	} else {
		delete(delayedApns, registration)
	}
	mapMutex.Unlock()
	log.Debugln("Sending notification to", registration.AccountId, "/", registration.DeviceToken)

	notification := &apns2.Notification{}
	notification.DeviceToken = registration.DeviceToken
	notification.Topic = topic
	composedPayload := []byte(`{"aps":{`)
	composedPayload = append(composedPayload, []byte(`"account-id":"` + registration.AccountId + `"`)...)
	composedPayload = append(composedPayload, []byte(`}}`)...)
	notification.Payload = composedPayload
	notification.ApnsID = "40636A2C-C093-493E-936A-2A4333C06DEA"
	notification.Expiration = time.Now().Add(24 * time.Hour)
	// set the apns-priority
	//notification.Priority = apns2.PriorityLow

	res, err := client.Push(notification)

	if err != nil {
		log.Fatal("Error:", err)
	}

	switch res.StatusCode {
	case http.StatusOK:
		log.Debugln("Apple returned 200 for notification to", registration.AccountId, "/", registration.DeviceToken)
	case 410:
		// The device token is inactive for the specified topic.
		log.Debugln("Apple returned 410 for notification to", registration.AccountId, "/", registration.DeviceToken)
		db.DeleteIfExistRegistration(registration.DeviceToken)
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
