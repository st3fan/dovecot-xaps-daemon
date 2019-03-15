package aps

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	log "github.com/sirupsen/logrus"
	"github.com/st3fan/dovecot-xaps-daemon/database"
	"github.com/timehop/apns"
	"io/ioutil"
	"sync"
	"time"
)

var oidUid = []int{0, 9, 2342, 19200300, 100, 1, 1}
var productionOID = []int{1, 2, 840, 113635, 100, 6, 3, 2}

var client apns.Client
var db *database.Database
var mapMutex = &sync.Mutex{}
var delayedApns = make(map[database.Registration]time.Time)
var delayTime = 30

func NewApns(certFile string, keyFile string, checkDelayedInterval int, delayMessageTime int, feedbackInterval int, database *database.Database) string {
	log.Debugln("Parsing", certFile, "to obtain APNS Topic")
	log.Debugln("APNS for non NewMessage events will be delayed for", time.Second*time.Duration(delayTime))
	delayTime = delayMessageTime
	db = database
	certtopic, err := topicFromCertificate(certFile)
	if err != nil {
		log.Fatalln("Could not parse apns topic from certificate: ", err)
	}
	log.Debugln("Topic is", certtopic)
	log.Debugln("Creating APNS client to", apns.ProductionGateway)

	client, err = apns.NewClientWithFiles(apns.ProductionGateway, certFile, keyFile)
	if err != nil {
		log.Fatal("Could not create client: ", err.Error())
	}

	// https://developer.apple.com/library/archive/documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/BinaryProviderAPI.html
	feedback, err := apns.NewFeedbackWithFiles(apns.ProductionFeedbackGateway, certFile, keyFile)
	if err != nil {
		log.Fatal("Could not create feedback service: ", err.Error())
	}
	if feedbackInterval > 0 {
		feedbackTicker := time.NewTicker(time.Minute * time.Duration(feedbackInterval))
		go func() {
			for range feedbackTicker.C {
				for f := range feedback.Receive() {
					db.DeleteIfExistRegistration(f.DeviceToken, f.Timestamp)
				}
			}
		}()
	}

	go func() {
		for f := range client.FailedNotifs {
			log.Println("Notification", f.Notif.ID, "failed with", f.Err.Error())
		}
	}()

	delayedNotificationTicker := time.NewTicker(time.Second * time.Duration(checkDelayedInterval))
	go func() {
		for range delayedNotificationTicker.C {
			checkDelayed()
		}
	}()

	return certtopic
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
	payload := apns.NewPayload()
	payload.APS.AccountId = registration.AccountId
	notification := apns.NewNotification()
	notification.Payload = payload
	notification.DeviceToken = registration.DeviceToken
	// set expiration
	// https://developer.apple.com/library/content/documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/CommunicatingwithAPNs.html
	t := time.Now().Add(24 * time.Hour)
	notification.Expiration = &t
	client.Send(notification)
}

func topicFromCertificate(filename string) (string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalln("Could not read file: ", err)
	}
	block, _ := pem.Decode([]byte(data))
	if block == nil {
		return "", errors.New("Could not decode PEM block from certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
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
