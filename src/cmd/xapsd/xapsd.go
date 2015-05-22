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
	"bufio"
	"errors"
	"flag"
	"fmt"
	"github.com/st3fan/apns"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

var notificationIdentifier uint32
var notificationIdentifierMutex sync.Mutex

func initializeNotificationIdentifier() {
	notificationIdentifier = uint32(time.Now().Unix())
}

func nextNotificationIdentifier() uint32 {
	notificationIdentifierMutex.Lock()
	defer notificationIdentifierMutex.Unlock()
	notificationIdentifier += 1
	return notificationIdentifier
}

//

type command struct {
	name string
	args map[string]interface{}
}

func (cmd *command) getStringArg(name string) (string, bool) {
	arg, ok := cmd.args[name].(string)
	return arg, ok
}

func (cmd *command) getListArg(name string) ([]string, bool) {
	arg, ok := cmd.args[name].([]string)
	return arg, ok
}

func parseListValue(value string) ([]string, error) {
	list := []string{}
	values := strings.Split(value[1:len(value)-1], ",")
	for _, value := range values {
		stringValue, err := parseStringValue(value)
		if err != nil {
			return nil, err
		}
		list = append(list, stringValue)
	}
	return list, nil
}

func parseStringValue(value string) (string, error) {
	return value[1 : len(value)-1], nil // TODO Escaping!
}

func parseCommand(line string) (command, error) {
	cmd := command{args: make(map[string]interface{})}

	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 {
		return cmd, errors.New("Failed to parse: no name found")
	}

	cmd.name = parts[0]

	for _, pair := range strings.Split(parts[1], "\t") {
		nameAndValue := strings.SplitN(pair, "=", 2)
		if len(nameAndValue) != 2 {
			return cmd, errors.New("Failed to parse: no name/value pair found")
		}

		switch {
		case strings.HasPrefix(nameAndValue[1], `"`) && strings.HasSuffix(nameAndValue[1], `"`):
			value, err := parseStringValue(nameAndValue[1])
			if err != nil {
				return cmd, err
			}
			cmd.args[nameAndValue[0]] = value
		case strings.HasPrefix(nameAndValue[1], "(") && strings.HasSuffix(nameAndValue[1], ")"):
			value, err := parseListValue(nameAndValue[1])
			if err != nil {
				return cmd, err
			}
			cmd.args[nameAndValue[0]] = value
		default:
			return cmd, errors.New("Failed to parse: invalid value in key/value pair")
		}
	}

	return cmd, nil
}

var socket = flag.String("socket", "/var/run/xapsd/xapsd.sock", "TODO")
var database = flag.String("database", "/var/lib/xapsd/database.json", "TODO")
var key = flag.String("key", "/etc/xapsd/key.pem", "TODO")

var certificate = flag.String("certificate", "/etc/xapsd/certificate.pem", "TODO")

//var apns = flag.String("apns", "gateway.push.apple.com:2195", "TODO")

func main() {
	flag.Parse()

	db, err := newDatabase(*database)
	if err != nil {
		log.Fatal("Cannot open database", *database, err.Error())
	}

	listener, err := net.Listen("unix", *socket)
	if err != nil {
		log.Fatal("Could not create socket", err.Error())
	}
	defer os.Remove(*socket)

	// TODO What is the proper way to limit Dovecot to this socket
	if err := os.Chmod(*socket, 0777); err != nil {
		log.Fatal("Could not chmod socket", err.Error())
	}

	fmt.Println("Listenening on", *socket)

	c, err := apns.NewClientWithFiles(apns.ProductionGateway, *certificate, *key)
	if err != nil {
		log.Fatal("Could not create client", err.Error())
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Failed to accept connection: ", err.Error())
			os.Exit(1)
		}

		go handleRequest(conn, &c, db)
	}
}

func handleRequest(conn net.Conn, client *apns.Client, db *Database) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {

		fmt.Println("INCOMING LINE", scanner.Text())

		command, err := parseCommand(scanner.Text())
		if err != nil {
			fmt.Fprintln(os.Stderr, "Reading froms socket: ", err)
		}

		switch command.name {
		case "REGISTER":
			handleRegister(conn, command, client, db)
		case "NOTIFY":
			handleNotify(conn, command, client, db)
		default:
			writeError(conn, "Unknown command")
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Reading froms socket: ", err)
	}

	fmt.Println("Done with connection %v", conn)
}

//
// Handle the REGISTER command. It looks as follows:
//
//  REGISTER aps-account-id="AAA" aps-device-token="BBB"
//     aps-subtopic="com.apple.mobilemail"
//     dovecot-username="stefan"
//     dovecot-mailboxes=("Inbox","Notes")
//
// The command returns the aps-topic, which is the common name of
// the certificate issued by OS X Server for email push
// notifications.
//

func handleRegister(conn net.Conn, cmd command, client *apns.Client, db *Database) {
	fmt.Println("TRACE", "handleRegister")
	// Make sure the subtopic is ok
	subtopic, ok := cmd.getStringArg("aps-subtopic")
	if !ok {
		writeError(conn, "Missing apis-subtopic argument")
		return
	}
	if subtopic != "com.apple.mobilemail" {
		writeError(conn, "Unknown aps-subtopic")
		return
	}

	// Make sure we got the required parameters
	accountId, ok := cmd.getStringArg("aps-account-id")
	if !ok {
		writeError(conn, "Missing aps-account-id argument")
		return
	}
	deviceToken, ok := cmd.getStringArg("aps-device-token")
	if !ok {
		writeError(conn, "Missing aps-device-token argument")
		return
	}
	username, ok := cmd.getStringArg("dovecot-username")
	if !ok {
		writeError(conn, "Missing dovecot-username argument")
		return
	}
	mailboxes, ok := cmd.getListArg("dovecot-mailboxes")
	if !ok {
		writeError(conn, "Missing dovecot-mailboxes argument")
		return
	}

	// Register this email/account-id/device-token combination
	fmt.Println("TRACE", " ", "addRegistration", username, accountId, deviceToken, mailboxes)
	err := db.addRegistration(username, accountId, deviceToken, mailboxes)
	if err != nil {
		writeError(conn, "Failed to register client: "+err.Error())
		return
	}

	writeSuccess(conn, "topic-name-here") // TODO
}

//
// Handle the NOTIFY command. It looks as follows:
//
//  NOTIFY dovecot-username="stefan" dovecot-mailbox="Inbox"
//
// See if the the username has devices registered. If it has, loop
// over them to find the ones that are interested in the named
// mailbox and send those a push notificiation.
//
// The push notification looks like this:
//
//  { "aps": { "account-id": aps-account-id } }
//

func handleNotify(conn net.Conn, cmd command, client *apns.Client, db *Database) {
	fmt.Println("TRACE", "handleNotify")

	// Make sure we got the required arguments
	username, ok := cmd.getStringArg("dovecot-username")
	if !ok {
		writeError(conn, "Missing dovecot-username argument")
		return
	}
	mailbox, ok := cmd.getStringArg("dovecot-mailbox")
	if !ok {
		writeError(conn, "Missing dovecot-mailbox argument")
		return
	}

	// Find all the devices registered for this mailbox event
	fmt.Println("TRACE", " ", "findRegistrations", username, mailbox)
	registrations, err := db.findRegistrations(username, mailbox)
	if err != nil {
		writeError(conn, "Cannot lookup registrations: "+err.Error())
		return
	}

	// Send a notification to all registered devices. We ignore failures
	// because there is not a lot we can do.
	for _, registration := range registrations {
		fmt.Println("Sending a notification to", registration.DeviceToken)
		sendNotification(registration, client)
	}

	writeSuccess(conn, "")
}

func sendNotification(reg Registration, client *apns.Client) {
	payload := apns.NewPayload()
	payload.APS.AccountId = reg.AccountId
	notification := apns.NewNotification()
	notification.Payload = payload
	notification.DeviceToken = reg.DeviceToken
	notification.Priority = apns.PriorityImmediate
	notification.Identifier = nextNotificationIdentifier()
	client.Send(notification)
}

func writeError(conn net.Conn, msg string) {
	conn.Write([]byte("ERR" + " " + msg + "\n"))
}

func writeSuccess(conn net.Conn, msg string) {
	conn.Write([]byte("OK" + " " + msg + "\n"))
}
