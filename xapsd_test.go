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

package main

import (
	"testing"
)

func Test_ParseCommand_Register(t *testing.T) {
	line := "REGISTER aps-account-id=\"AAA\"\taps-device-token=\"BBB\"\taps-subtopic=\"com.apple.mobilemail\"\tdovecot-username=\"stefan\"\tdovecot-mailboxes=(\"Inbox\",\"Notes\")"

	cmd, err := parseCommand(line)
	if err != nil {
		t.Error("Cannot parseCommand", err)
	}

	if cmd.name != "REGISTER" {
		t.Error(`cmd.name != "REGISTER"`)
	}

	if val, _ := cmd.getStringArg("aps-account-id"); val != "AAA" {
		t.Error(`val != "AAA" ` + val)
	}

	if val, _ := cmd.getStringArg("aps-device-token"); val != "BBB" {
		t.Error(`val != "BBB"` + val)
	}

	if val, _ := cmd.getStringArg("aps-subtopic"); val != "com.apple.mobilemail" {
		t.Error(`val != "com.apple.mobilemail"` + val)
	}

	if val, _ := cmd.getStringArg("dovecot-username"); val != "stefan" {
		t.Error(`val != "stefan"` + val)
	}

	if val, ok := cmd.getListArg("dovecot-mailboxes"); !ok || len(val) != 2 || val[0] != "Inbox" || val[1] != "Notes" {
		t.Error(`Cannot getListArg("dovecot-mailboxes")`)
	}
}

func Test_ParseCommand_Notify(t *testing.T) {
	line := "NOTIFY dovecot-username=\"stefan\"\tdovecot-mailbox=\"Inbox\""

	cmd, err := parseCommand(line)
	if err != nil {
		t.Error("Cannot parseCommand", err)
	}

	if cmd.name != "NOTIFY" {
		t.Error(`cmd.name != "NOTIFY"`)
	}

	if val, _ := cmd.getStringArg("dovecot-username"); val != "stefan" {
		t.Error(`val != "stefan" ` + val)
	}

	if val, _ := cmd.getStringArg("dovecot-mailbox"); val != "Inbox" {
		t.Error(`val != "Inbox" ` + val)
	}
}
