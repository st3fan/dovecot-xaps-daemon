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
	"io"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

func DBCreateWorkingCopy() {
	// Open original file
	original, err := os.Open("testdata/database.json")
	if err != nil {
		log.Fatal(err)
	}
	defer original.Close()

	// Create cpy file
	cpy, err := os.Create("testdata/database_workingcpy.json")
	if err != nil {
		log.Fatal(err)
	}
	defer cpy.Close()

	//This will copy
	io.Copy(cpy, original)
}

func TestDatabase_NewDatabase(t *testing.T) {
	DBCreateWorkingCopy()
	db, err := NewDatabase("testdata/database_workingcpy.json")
	if err != nil {
		t.Error("Cannot open database testdata/database_workingcpy.json", err)
	}

	if db.filename != "testdata/database_workingcpy.json" {
		t.Error(`db.filename != "testdata/database_workingcpy.json"`)
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

func TestDatabase_AddRegistration(t *testing.T) {
	f, err := ioutil.TempFile("/tmp", "database_test_Test_addRegistration")
	if err != nil {
		t.Error("Can't create temporary file", err)
	}
	defer os.Remove(f.Name())

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

	db, err = NewDatabase(f.Name())
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

func TestDatabase_FindRegistrations(t *testing.T) {
	DBCreateWorkingCopy()
	db, err := NewDatabase("testdata/database_workingcpy.json")
	if err != nil {
		t.Error("Cannot open database testdata/database_workingcpy.json", err)
	}

	registrations, err := db.FindRegistrations("stefan", "Inbox")
	if err != nil {
		t.Error("Cannot findRegistrations:", err)
	}

	if len(registrations) != 2 {
		t.Error(`len(registrations) != 2`)
	}

	registrations, err = db.FindRegistrations("stefan", "Ham")
	if err != nil {
		t.Error("Cannot findRegistrations:", err)
	}

	if len(registrations) != 1 {
		t.Error(`len(registrations) != 1`)
	}

	registrations, err = db.FindRegistrations("doesnotexist", "Inbox")
	if err != nil {
		t.Error("Cannot findRegistrations:", err)
	}

	if len(registrations) != 0 {
		t.Error(`len(registrations) != 0`)
	}
}

func TestDatabase_AccountContainsMailbox(t *testing.T) {
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

func TestDatabase_DeleteIfExistRegistration(t *testing.T) {
	DBCreateWorkingCopy()
	db, err := NewDatabase("testdata/database_workingcpy.json")
	if err != nil {
		t.Error("Cannot open database testdata/database_workingcpy.json", err)
	}

	success := db.DeleteIfExistRegistration("alicedevicetoken1")
	if !success {
		t.Error("Device token could not be removed", err)
	}

	success = db.DeleteIfExistRegistration("alicedevicetoken1")
	if success {
		t.Error("Not existend device token has been *successfully* deleted???", err)
	}
}

func TestDatabase_CleanupRegistration(t *testing.T) {
	DBCreateWorkingCopy()
	db, err := NewDatabase("testdata/database_workingcpy.json")
	if err != nil {
		t.Error("Cannot open database testdata/database_workingcpy.json", err)
	}

	arr, _ := db.FindRegistrations("alice", "Inbox")
	if len(arr) < 1 {
		t.Error("Registration to cleanup not found!")
	}

	db.cleanupRegistered()

	arr, _ = db.FindRegistrations("alice", "Inbox")
	if len(arr) > 0 {
		t.Error("Registration not cleaned up!")
	}
}
