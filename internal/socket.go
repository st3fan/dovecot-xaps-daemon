package internal

import (
	"bufio"
	"errors"
	"github.com/freswa/dovecot-xaps-daemon/internal/config"
	"github.com/freswa/dovecot-xaps-daemon/internal/database"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"strings"
)

type command struct {
	name string
	args map[string]interface{}
}

func NewSocket(config *config.Config, db *database.Database, apns *Apns) {
	// Delete the socketpath if it already exists
	if _, err := os.Stat(config.SocketPath); err == nil {
		err := os.Remove(config.SocketPath)
		if err != nil {
			log.Fatalln("Could not delete existing socketpath: ", config.SocketPath, err)
		}
	}
	log.Debugln("Listening on UNIX socketpath at", config.SocketPath)

	listener, err := net.Listen("unix", config.SocketPath)
	if err != nil {
		log.Fatalln("Could not create socketpath: ", err)
	}
	defer os.Remove(config.SocketPath)

	// TODO What is the proper way to limit Dovecot to this socketpath
	err = os.Chmod(config.SocketPath, 0777)
	if err != nil {
		log.Fatalln("Could not chmod socketpath: ", err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Failed to accept connection: ", err.Error())
			os.Exit(1)
		}

		log.Debugln("Accepted a connection")
		go handleRequest(conn, db, apns)
	}
}

func parseListValue(value string) ([]string, error) {
	var list []string
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

func handleRequest(conn net.Conn, db *database.Database, apns *Apns) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		log.Debugln("Received request:", scanner.Text())

		command, err := parseCommand(scanner.Text())
		if err != nil {
			log.Fatalln("Error parsing socket data: ", err)
		}

		switch command.name {
		case "REGISTER":
			handleRegister(conn, command, db, apns)
		case "NOTIFY":
			handleNotify(conn, command, db, apns)
		default:
			writeError(conn, "Unknown command")
		}
	}

	err := scanner.Err()
	if err != nil {
		log.Fatalln("Error while reading from socket: ", err)
	}
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
func handleRegister(conn net.Conn, cmd command, db *database.Database, apns *Apns) {
	// Make sure the subtopic is ok
	subtopic, ok := cmd.getStringArg("aps-subtopic")
	if !ok {
		writeError(conn, "Missing aps-subtopic argument")
	}
	if subtopic != "com.apple.mobilemail" {
		writeError(conn, "Unknown aps-subtopic")
	}

	// Make sure we got the required parameters
	accountId, ok := cmd.getStringArg("aps-account-id")
	if !ok {
		writeError(conn, "Missing aps-account-id argument")
	}
	deviceToken, ok := cmd.getStringArg("aps-device-token")
	if !ok {
		writeError(conn, "Missing aps-device-token argument")
	}
	username, ok := cmd.getStringArg("dovecot-username")
	if !ok {
		writeError(conn, "Missing dovecot-username argument")
	}
	mailboxes, ok := cmd.getListArg("dovecot-mailboxes")
	if !ok {
		writeError(conn, "Missing dovecot-mailboxes argument")
	}
	// Register this email/account-id/device-token combination
	err := db.AddRegistration(username, accountId, deviceToken, mailboxes)
	if !ok {
		writeError(conn, "Failed to register client: "+err.Error())
	}
	writeSuccess(conn, apns.Topic)
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
func handleNotify(conn net.Conn, cmd command, db *database.Database, apns *Apns) {
	// Make sure we got the required arguments
	username, ok := cmd.getStringArg("dovecot-username")
	if !ok {
		writeError(conn, "Missing dovecot-username argument")
	}

	mailbox, ok := cmd.getStringArg("dovecot-mailbox")
	if !ok {
		writeError(conn, "Missing dovecot-mailbox argument")
	}

	isMessageNew := false
	events, ok := cmd.getListArg("events")
	if !ok {
		log.Warnln("No events found in NOTIFY message, please update the xaps-dovecot-plugin!")
		isMessageNew = true
	}

	// check if this is an event for a new message
	// for all possible events have a look at dovecot-core:
	// grep '#define EVENT_NAME' src/plugins/push-notification/push-notification-event*
	for _, e := range events {
		if e == "MessageNew" {
			isMessageNew = true
		}
	}

	// we don't know how to handle other mboxes other than INBOX, so ignore them
	if mailbox != "INBOX" {
		log.Debugln("Ignoring non INBOX event for:", mailbox)
		writeSuccess(conn, "")
		return
	}

	// Find all the devices registered for this mailbox event
	registrations, err := db.FindRegistrations(username, mailbox)
	if err != nil {
		writeError(conn, "Cannot lookup registrations: "+err.Error())
	}

	for _, r := range registrations {
		log.Debugf("Found registration %s with token %s for username: %s", r.AccountId, r.DeviceToken, username)
	}
	if len(registrations) == 0 {
		log.Debugf("No registration found for username: %s", username)
	}

	// Send a notification to all registered devices. We ignore failures
	// because there is not a lot we can do.
	for _, registration := range registrations {
		apns.SendNotification(registration, !isMessageNew)
	}
	writeSuccess(conn, "")
}

func (cmd *command) getStringArg(name string) (string, bool) {
	arg, ok := cmd.args[name].(string)
	return arg, ok
}

func (cmd *command) getListArg(name string) ([]string, bool) {
	arg, ok := cmd.args[name].([]string)
	return arg, ok
}

func writeError(conn net.Conn, msg string) {
	log.Debugln("Returning failure:", msg)
	conn.Write([]byte("ERROR" + " " + msg + "\n"))
}

func writeSuccess(conn net.Conn, msg string) {
	log.Debugln("Returning success:", msg)
	conn.Write([]byte("OK" + " " + msg + "\n"))
}
