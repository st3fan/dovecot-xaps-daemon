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
	"io/ioutil"
	"os"
	"testing"
)

func Test_newDatabase(t *testing.T) {
	db, err := NewDatabase("testdata/database.json")
	if err != nil {
		t.Error("Cannot open database testdata/database.json", err)
	}

	if db.filename != "testdata/database.json" {
		t.Error(`db.filename != "testdata/database.json"`)
	}

	if len(db.Users) != 2 {
		t.Error("len(db.Users) != 2")
	}

	if len(db.Users["stefan"].Accounts) != 2 {
		t.Error(`db.Users["stefan"].Accounts) != 1`)
	}

	if len(db.Users["alice"].Accounts) != 1 {
		t.Error(`db.Users["alice"].Accounts) != 1`)
	}
}

func Test_addRegistration(t *testing.T) {
	f, err := ioutil.TempFile("/tmp", "database_test_Test_addRegistration")
	if err != nil {
		t.Error("Can't create temporary file", err)
	}
	defer os.Remove(f.Name())

	if true {
		db, err := NewDatabase(f.Name())
		if err != nil {
			t.Error("Cannot open database", err)
		}

		if err := db.AddRegistration("test@example.com", "testaccountid1", "testtoken1", []string{"Inbox", "Spam"}); err != nil {
			t.Error("Cannot addRegistration:", err)
		}

		if err := db.AddRegistration("test@example.com", "testaccountid2", "testtoken2", []string{"Inbox", "Ham"}); err != nil {
			t.Error("Cannot addRegistration:", err)
		}

		if err := db.AddRegistration("alice@example.com", "aliceaccountid", "alicetoken", []string{"Inbox", "Important"}); err != nil {
			t.Error("Cannot addRegistration:", err)
		}
	}

	if true {
		db, err := NewDatabase(f.Name())
		if err != nil {
			t.Error("Cannot open database", err)
		}

		if _, ok := db.Users["test@example.com"]; !ok {
			t.Error(`Cannot find Users["test@example.com"]`)
		}

		if _, ok := db.Users["alice@example.com"]; !ok {
			t.Error(`Cannot find Users["test@example.com"]`)
		}
	}
}

func Test_findRegistrations(t *testing.T) {
	db, err := NewDatabase("testdata/database.json")
	if err != nil {
		t.Error("Cannot open database testdata/database.json", err)
	}

	if true {
		registrations, err := db.FindRegistrations("stefan", "Inbox")
		if err != nil {
			t.Error("Cannot findRegistrations:", err)
		}

		if len(registrations) != 2 {
			t.Error(`len(registrations) != 2`)
		}
	}

	if true {
		registrations, err := db.FindRegistrations("stefan", "Ham")
		if err != nil {
			t.Error("Cannot findRegistrations:", err)
		}

		if len(registrations) != 1 {
			t.Error(`len(registrations) != 1`)
		}
	}

	if true {
		registrations, err := db.FindRegistrations("doesnotexist", "Inbox")
		if err != nil {
			t.Error("Cannot findRegistrations:", err)
		}

		if len(registrations) != 0 {
			t.Error(`len(registrations) != 0`)
		}
	}
}

func Test_Account_ContainsMailbox(t *testing.T) {
	account := Account{DeviceToken: "SomeToken", Mailboxes: []string{"Inbox", "Ham"}}

	if account.ContainsMailbox("Inbox") != true {
		t.Error(`account.ContainsMailbox("Inbox") != true`)
	}

	if account.ContainsMailbox("Ham") != true {
		t.Error(`account.ContainsMailbox("Ham") != true`)
	}

	if account.ContainsMailbox("Cheese") != false {
		t.Error(`account.ContainsMailbox("Cheese") != false`)
	}
}
