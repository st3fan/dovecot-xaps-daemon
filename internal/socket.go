package internal

import (
	"encoding/json"
	"github.com/freswa/dovecot-xaps-daemon/internal/config"
	"github.com/freswa/dovecot-xaps-daemon/internal/database"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type httpHandler struct {
	db   *database.Database
	apns *Apns
}

// REGISTER aps-account-id="AAA" aps-device-token="BBB"
//    aps-subtopic="com.apple.mobilemail"
//    dovecot-username="stefan"
//    dovecot-mailboxes=("Inbox","Notes")
type Register struct {
	ApsAccountId   string
	ApsDeviceToken string
	ApsSubtopic    string
	Username       string
	Mailboxes      []string
}

// NOTIFY dovecot-username="stefan" dovecot-mailbox="Inbox"
type Notify struct {
	Username string
	Mailbox  string
	Events   []string
}

func NewHttpSocket(config *config.Config, db *database.Database, apns *Apns) {
	router := httprouter.New()
	httpSocket := httpHandler{db, apns}
	router.POST("/register", httpSocket.handleRegister)
	router.POST("/notify", httpSocket.handleNotify)
	err := http.ListenAndServe(":"+config.Port, router)
	if err != nil {
		log.Fatalf("Could not listen on Port %s: %s", config.Port, err)
	}
	log.Infof("Listening on Port %s", config.Port)
}

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
func (httpHandler *httpHandler) handleRegister(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	defer request.Body.Close()

	reg := Register{}
	err := json.NewDecoder(request.Body).Decode(&reg)
	if err != nil {
		log.Errorf("Error while handling register call: %s", err)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	if reg.checkParams() {
		log.Errorf("Incomplete register payload: %v", reg)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	// Make sure the subtopic is ok
	if reg.ApsSubtopic != "com.apple.mobilemail" {
		log.Errorf("Unknown aps-subtopic: %s", reg.ApsSubtopic)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	// Register this email/account-id/device-token combination
	err = httpHandler.db.AddRegistration(reg.Username, reg.ApsAccountId, reg.ApsDeviceToken, reg.Mailboxes)
	if err != nil {
		log.Errorf("Failed to register client:: %s", err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	writer.Write([]byte(httpHandler.apns.Topic))
}

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
func (httpHandler *httpHandler) handleNotify(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	defer request.Body.Close()

	notify := Notify{}
	err := json.NewDecoder(request.Body).Decode(&notify)
	if err != nil {
		log.Errorf("Error while handling notify call: %s", err)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	if notify.checkParams() {
		log.Errorf("Incomplete register payload: %v", notify)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	isMessageNew := false
	// check if this is an event for a new message
	// for all possible events have a look at dovecot-core:
	// grep '#define EVENT_NAME' src/plugins/push-notification/push-notification-event*
	for _, e := range notify.Events {
		if e == "MessageNew" {
			isMessageNew = true
		}
	}

	// we don't know how to handle other mailboxes other than INBOX, so ignore them
	if notify.Mailbox != "INBOX" {
		log.Debugln("Ignoring non INBOX event for:", notify.Mailbox)
		writer.WriteHeader(http.StatusOK)
		return
	}

	// Find all the devices registered for this mailbox event
	registrations, err := httpHandler.db.FindRegistrations(notify.Username, notify.Mailbox)
	if err != nil {
		log.Errorf("Cannot lookup registrations: %s", err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, r := range registrations {
		log.Debugf("Found registration %s with token %s for username: %s", r.AccountId, r.DeviceToken, notify.Username)
	}
	if len(registrations) == 0 {
		if httpHandler.db.UserExists(notify.Username) {
			// This isn't an error as registrations are also empty if the mailbox doesn't match
			log.Infof("No registered mailbox found for username: %s", notify.Username)
			writer.WriteHeader(http.StatusNoContent)
		} else {
			log.Warnf("No registration found for username: %s", notify.Username)
			writer.WriteHeader(http.StatusNotFound)
		}
		return
	}

	// Send a notification to all registered devices. We ignore failures
	// because there is not a lot we can do.
	for _, registration := range registrations {
		httpHandler.apns.SendNotification(registration, !isMessageNew)
	}

	writer.WriteHeader(http.StatusOK)
}

func (reg *Register) checkParams() (isError bool) {
	// Make sure we got the required parameters
	if len(reg.ApsAccountId) == 0 {
		log.Error("Missing aps-account-id in register request")
		isError = true
	}
	if len(reg.ApsDeviceToken) == 0 {
		log.Error("Missing aps-device-token in register request")
		isError = true
	}
	if len(reg.Username) == 0 {
		log.Error("Missing dovecot-username in register request")
		isError = true
	}
	if len(reg.Mailboxes) == 0 {
		log.Error("Missing dovecot-mailboxes in register request")
		isError = true
	}
	return
}

func (notify *Notify) checkParams() (isError bool) {
	// Make sure we got the required parameters
	if len(notify.Username) == 0 {
		log.Error("Missing username in notify request")
		isError = true
	}
	if len(notify.Mailbox) == 0 {
		log.Error("Missing mailbox in notify request")
		isError = true
	}
	if len(notify.Events) == 0 {
		log.Error("Missing register in notify request")
		isError = true
	}
	return
}
