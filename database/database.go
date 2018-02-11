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
	"io/ioutil"
	"os"
	"sync"
)

var dbMutex = &sync.Mutex{}

type Registration struct {
	DeviceToken string
	AccountId   string
}

type Account struct {
	//AccountId     string
	DeviceToken string
	Mailboxes   []string
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
	//Username string
	Accounts map[string]Account
}

type Database struct {
	filename string
	Users    map[string]User
}

func NewDatabase(filename string) (*Database, error) {
	// TODO This is not awesome.
	// Let's rewrite. Like replace this with Open(..., "rw") instead of ReadFile()
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		db := &Database{filename: filename, Users: make(map[string]User)}
		if err := db.write(); err != nil {
			return nil, err
		}
		return db, nil
	} else if err != nil {
		return nil, err
	} else {
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}

		db := Database{filename: filename, Users: make(map[string]User)}
		if len(data) != 0 {
			if err := json.Unmarshal(data, &db); err != nil {
				return nil, err
			}
		}

		return &db, nil
	}
}

func (db *Database) write() error {
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(db.filename, data, 0644)
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
	db.Users[username].Accounts[accountId] = Account{DeviceToken: deviceToken, Mailboxes: mailboxes}

	err := db.write()

	// release mutex
	dbMutex.Unlock()
	return err
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
