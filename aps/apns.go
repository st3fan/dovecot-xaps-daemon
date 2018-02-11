package aps

import (
	log "github.com/sirupsen/logrus"
	"encoding/pem"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"github.com/timehop/apns"
	"time"
)


var client apns.Client

func NewApns(certFile string, keyFile string) string {
	log.Debugln("Parsing", certFile, "to obtain APNS Topic")

	certtopic, err := topicFromCertificate(certFile)
	if err != nil {
		log.Fatalln("Could not parse apns topic from certificate: ", err)
	}
	log.Debugln("Topic is", certtopic)
	log.Debugln("Creating APNS client to", apns.ProductionGateway)

	c, err := apns.NewClientWithFiles(apns.ProductionGateway, certFile, keyFile)
	if err != nil {
		log.Fatal("Could not create client: ", err.Error())
	}
	client = c

	go func() {
		for f := range c.FailedNotifs {
			log.Println("Notification", f.Notif.ID, "failed with", f.Err.Error())
		}
	}()

	return certtopic
}

func SendNotification(accountId string, deviceToken string) {
	payload := apns.NewPayload()
	payload.APS.AccountId = accountId
	notification := apns.NewNotification()
	notification.Payload = payload
	notification.DeviceToken = deviceToken
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

	oidUid := []int{0, 9, 2342, 19200300, 100, 1, 1}
	if !cert.Subject.Names[0].Type.Equal(oidUid) {
		return "", errors.New("Did not find a Subject.Names[0] with type 0.9.2342.19200300.100.1.1")
	}

	return cert.Subject.Names[0].Value.(string), nil
}