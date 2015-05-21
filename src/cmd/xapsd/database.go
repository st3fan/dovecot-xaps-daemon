//
// The MIT License (MIT)
//
// Copyright (c) 2015 Stefan Arentz <stefan@arentz.ca>
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

package main

import (
	"encoding/json"
	"io/ioutil"
)

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

func newDatabase(filename string) (*Database, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var db Database = Database{filename: filename, Users: make(map[string]User)}
	if len(data) != 0 {
		if err := json.Unmarshal(data, &db); err != nil {
			return nil, err
		}
	}

	return &db, nil
}

func (db *Database) write() error {
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(db.filename, data, 0644)
}

func (db *Database) addRegistration(username, accountId, deviceToken string, mailboxes []string) error {
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

	return db.write()
}

func (db *Database) findRegistrations(username, mailbox string) ([]Registration, error) {
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
