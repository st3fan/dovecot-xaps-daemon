package internal

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/freswa/dovecot-xaps-daemon/internal/config"
	"github.com/freswa/dovecot-xaps-daemon/internal/database"
	"github.com/sideshow/apns2"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	oidUid        = []int{0, 9, 2342, 19200300, 100, 1, 1}
	productionOID = []int{1, 2, 840, 113635, 100, 6, 3, 2}
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
	log.Debugln("Parsing", config.CertFile, "to obtain APNS Topic")
	certData, keyData, filesExist, err := loadCertificate(config)
	if filesExist && err != nil {
		log.Fatalln("Could not load certificate: ", err)
	}
	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		log.Fatalln("Could not parse certificate: ", err)
	}
	apns.Topic, err = topicFromCertificate(cert)
	if err != nil {
		log.Fatalln("Could not parse apns topic from certificate: ", err)
	}
	log.Debugln("Topic is", apns.Topic)
	apns.client = apns2.NewClient(cert).Production()
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
	}
	apns.mapMutex.Unlock()
	log.Debugln("Sending notification to", registration.AccountId, "/", registration.DeviceToken)

	notification := &apns2.Notification{}
	notification.DeviceToken = registration.DeviceToken
	notification.Topic = apns.Topic
	composedPayload := []byte(`{"aps":{`)
	composedPayload = append(composedPayload, []byte(`"account-id":"`+registration.AccountId+`"`)...)
	composedPayload = append(composedPayload, []byte(`}}`)...)
	notification.Payload = composedPayload
	notification.ApnsID = "40636A2C-C093-493E-936A-2A4333C06DEA"
	notification.Expiration = time.Now().Add(24 * time.Hour)
	// set the apns-priority
	//notification.Priority = apns2.PriorityLow

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

func loadCertificate(config *config.Config) (certData, keyData []byte, exists bool, err error) {
	// we assume the config is wrong
	exists = false
	// check if the config sections have been filled
	if len(config.CertFile) > 0 && len(config.KeyFile) > 0 {
		// then check if the file exist on disk
		exists = fileExists(config.CertFile) && fileExists(config.KeyFile)
	}
	// read
	certData, err = ioutil.ReadFile(config.CertFile)
	if err != nil {
		return
	}
	keyData, err = ioutil.ReadFile(config.KeyFile)
	return
}


func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}