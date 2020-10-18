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
	filename string
	Users    map[string]User
	Certs    apple_xserver_certs.Certificates
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

	return ioutil.WriteFile(db.filename, data, 0644)
}

func (db *Database) GetCerts() (certs *apple_xserver_certs.Certificates, success bool) {
	dbMutex.Lock()
	success = false
	if db.Certs.Mail != nil {
		certsCopy := db.Certs
		certs = &certsCopy
		success = true
	}
	dbMutex.Unlock()
	return
}

func (db *Database) PutCerts(certs apple_xserver_certs.Certificates) (err error) {
	dbMutex.Lock()
	db.Certs = certs
	err = db.write()
	dbMutex.Unlock()
	return
}

func (db *Database) AddRegistration(username, accountId, deviceToken string, mailboxes []string) error {
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

	err := db.write()

	// release mutex
	dbMutex.Unlock()
	return err
}

func (db *Database) DeleteIfExistRegistration(deviceToken string) bool {
	for _, user := range db.Users {
		for accountId, account := range user.Accounts {
			if account.DeviceToken == deviceToken {
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

func (db *Database) cleanupRegistered() {
	log.Debugln("Check Database for devices not calling IMAP hook for more than 30d")
	for _, user := range db.Users {
		for _, account := range user.Accounts {
			if !account.RegistrationTime.IsZero() && account.RegistrationTime.Before(time.Now().Add(-time.Hour*24*30)) {
				db.DeleteIfExistRegistration(account.DeviceToken)
			}
		}
	}
}
