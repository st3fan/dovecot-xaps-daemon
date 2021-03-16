//
// The MIT License (MIT)
//
// Copyright (c) 2015 Stefan Arentz <stefan@arentz.ca>
// Copyright (c) 2017 Frederik Schwan <frederik dot schwan at linux dot com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
//

package database

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"github.com/freswa/dovecot-xaps-daemon/pkg/apple_xserver_certs"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"sync"
	"time"
)

var dbMutex = &sync.Mutex{}

type Registration struct {
	DeviceToken string
	AccountId   string
}

type Account struct {
	DeviceToken      string
	Mailboxes        []string
	RegistrationTime time.Time
}

func (account *Account) ContainsMailbox(mailbox string) bool {
	for _, m := range account.Mailboxes {
		if m == mailbox {
			return true
		}
	}
	return false
}

type User struct {
	Accounts map[string]Account
}

type Database struct {
	filename   string
	Users      map[string]User
	AppleCerts DbCerts
	lastWrite time.Time
}

type DbCerts struct {
	Signature []tls.Certificate
	Keys      [][]byte
}

func NewDatabase(filename string) (*Database, error) {
	// check if file exists
	_, err := os.Stat(filename)
	if err != nil && os.IsNotExist(err) {
		db := &Database{filename: filename, Users: make(map[string]User)}
		err := db.write()
		if err != nil {
			return nil, err
		}
		return db, nil
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	db := Database{filename: filename, Users: make(map[string]User)}
	if len(data) != 0 {
		err := json.Unmarshal(data, &db)
		if err != nil {
			return nil, err
		}
	}

	registrationCleanupTicker := time.NewTicker(time.Hour * 8)
	go func() {
		for range registrationCleanupTicker.C {
			db.cleanupRegistered()
		}
	}()

	return &db, nil
}

func (db *Database) write() error {
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(db.filename + ".new", data, 0644)
	return os.Rename(db.filename + ".new", db.filename)
}

func (db *Database) GetCerts() (certs *apple_xserver_certs.Certificates, success bool) {
	dbMutex.Lock()
	success = false
	if db.AppleCerts.Signature != nil {
		calendarKey, err := x509.ParsePKCS1PrivateKey(db.AppleCerts.Keys[0])
		if err != nil {
			log.Fatal("Error while parsing calendar private key: ")
		}
		calendarCert := db.AppleCerts.Signature[0]
		calendarCert.PrivateKey = calendarKey
		contactKey, err := x509.ParsePKCS1PrivateKey(db.AppleCerts.Keys[1])
		if err != nil {
			log.Fatal("Error while parsing contact private key: ")
		}
		contactCert := db.AppleCerts.Signature[1]
		contactCert.PrivateKey = contactKey
		mailKey, err := x509.ParsePKCS1PrivateKey(db.AppleCerts.Keys[2])
		if err != nil {
			log.Fatal("Error while parsing mail private key: ")
		}
		mailCert := db.AppleCerts.Signature[2]
		mailCert.PrivateKey = mailKey
		mgmtKey, err := x509.ParsePKCS1PrivateKey(db.AppleCerts.Keys[3])
		if err != nil {
			log.Fatal("Error while parsing mgmt private key: ")
		}
		mgmtCert := db.AppleCerts.Signature[3]
		mgmtCert.PrivateKey = mgmtKey
		alertsKey, err := x509.ParsePKCS1PrivateKey(db.AppleCerts.Keys[4])
		if err != nil {
			log.Fatal("Error while parsing alerts private key: ")
		}
		alertsCert := db.AppleCerts.Signature[4]
		alertsCert.PrivateKey = alertsKey

		certs = &apple_xserver_certs.Certificates{
			Calendar: &calendarCert,
			Contact:  &contactCert,
			Mail:     &mailCert,
			Mgmt:     &mgmtCert,
			Alerts:   &alertsCert,
		}

		success = true
	}
	dbMutex.Unlock()
	return
}

func (db *Database) PutCerts(certs *apple_xserver_certs.Certificates) {
	dbMutex.Lock()

	db.AppleCerts.Keys = make([][]byte, 5)
	db.AppleCerts.Signature = make([]tls.Certificate, 5)

	calendarCert := *certs.Calendar
	contactCert := *certs.Contact
	mailCert := *certs.Mail
	mgmtCert := *certs.Mgmt
	alertsCert := *certs.Alerts

	db.AppleCerts.Keys[0] = x509.MarshalPKCS1PrivateKey(calendarCert.PrivateKey.(*rsa.PrivateKey))
	db.AppleCerts.Keys[1] = x509.MarshalPKCS1PrivateKey(contactCert.PrivateKey.(*rsa.PrivateKey))
	db.AppleCerts.Keys[2] = x509.MarshalPKCS1PrivateKey(mailCert.PrivateKey.(*rsa.PrivateKey))
	db.AppleCerts.Keys[3] = x509.MarshalPKCS1PrivateKey(mgmtCert.PrivateKey.(*rsa.PrivateKey))
	db.AppleCerts.Keys[4] = x509.MarshalPKCS1PrivateKey(alertsCert.PrivateKey.(*rsa.PrivateKey))

	calendarCert.PrivateKey = nil
	contactCert.PrivateKey = nil
	mailCert.PrivateKey = nil
	mgmtCert.PrivateKey = nil
	alertsCert.PrivateKey = nil

	db.AppleCerts.Signature[0] = calendarCert
	db.AppleCerts.Signature[1] = contactCert
	db.AppleCerts.Signature[2] = mailCert
	db.AppleCerts.Signature[3] = mgmtCert
	db.AppleCerts.Signature[4] = alertsCert

	err := db.write()
	if err != nil {
		log.Fatalln("Could not write database to disk while trying to save the certificates: ", err)
	}
	dbMutex.Unlock()
	return
}

func (db *Database) AddRegistration(username, accountId, deviceToken string, mailboxes []string) (err error) {
	//  mutual write access to database issue #16 xaps-plugin
	dbMutex.Lock()

	// Ensure the User exists
	if _, ok := db.Users[username]; !ok {
		db.Users[username] = User{Accounts: make(map[string]Account)}
	}

	// Ensure the Account exists
	if _, ok := db.Users[username].Accounts[accountId]; !ok {
		db.Users[username].Accounts[accountId] = Account{}
	}

	// Set or update the Registration
	db.Users[username].Accounts[accountId] =
		Account{
			DeviceToken:      deviceToken,
			Mailboxes:        mailboxes,
			RegistrationTime: time.Now(),
		}

	if db.lastWrite.Before(time.Now().Add(-time.Minute*15)) {
		err = db.write()
		db.lastWrite = time.Now()
	}

	// release mutex
	dbMutex.Unlock()
	return
}

func (db *Database) DeleteIfExistRegistration(reg Registration) bool {
	for _, user := range db.Users {
		for accountId, account := range user.Accounts {
			if accountId == reg.AccountId {
				dbMutex.Lock()
				log.Infoln("Deleting " + account.DeviceToken)
				delete(user.Accounts, accountId)
				db.write()
				dbMutex.Unlock()
				return true
			}
		}
	}
	return false
}

func (db *Database) FindRegistrations(username, mailbox string) ([]Registration, error) {
	var registrations []Registration
	if user, ok := db.Users[username]; ok {
		for accountId, account := range user.Accounts {
			if account.ContainsMailbox(mailbox) {
				registrations = append(registrations,
					Registration{DeviceToken: account.DeviceToken, AccountId: accountId})
			}
		}
	}
	return registrations, nil
}

func (db *Database) UserExists(username string) bool {
	_, ok := db.Users[username]
	return ok
}

func (db *Database) cleanupRegistered() {
	log.Debugln("Check Database for devices not calling IMAP hook for more than 30d")
	toDelete := make([]Registration, 0)
	dbMutex.Lock()
	for _, user := range db.Users {
		for accountId, account := range user.Accounts {
			if !account.RegistrationTime.IsZero() && account.RegistrationTime.Before(time.Now().Add(-time.Hour*24*30)) {
				toDelete = append(toDelete, Registration{account.DeviceToken, accountId})
			}
		}
	}
	dbMutex.Unlock()
	for _, reg := range toDelete {
		db.DeleteIfExistRegistration(reg)
	}
}
