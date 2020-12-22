package apple_xserver_certs

import (
	"bytes"
	"encoding/pem"
	"io/ioutil"
	"log"
	"net/http"
)

func NewCerts(username string, passwordhash string) *Certificates {
	certs := &Certificates{}
	generatePrivateKeys(certs)
	return requestCerts(certs, username, passwordhash)
}

func RenewCerts(certs *Certificates, username string, passwordhash string) *Certificates {
	return requestCerts(certs, username, passwordhash)
}

func requestCerts(certs *Certificates, username string, passwordhash string) *Certificates {
	body := createCertRequestBody(certs, username, passwordhash)
	response := sendRequest(body)
	responseBody, err := parseResponse(response)
	if err != nil {
		log.Fatal(err)
	}
	if responseBody.Response.Status.ErrorCode != 0 {
		log.Fatalf("Error %d while retrieving certificates:\n%+v", responseBody.Response.Status.ErrorCode, responseBody)
	}
	calendarCertDER, _ := pem.Decode([]byte(responseBody.Response.Certificates[0].Certificate))
	certs.Calendar.Certificate = make([][]byte, 1)
	certs.Calendar.Certificate[0] = calendarCertDER.Bytes
	contactCertDER, _ := pem.Decode([]byte(responseBody.Response.Certificates[1].Certificate))
	certs.Contact.Certificate = make([][]byte, 1)
	certs.Contact.Certificate[0] = contactCertDER.Bytes
	mailCertDER, _ := pem.Decode([]byte(responseBody.Response.Certificates[2].Certificate))
	certs.Mail.Certificate = make([][]byte, 1)
	certs.Mail.Certificate[0] = mailCertDER.Bytes
	mgmtCertDER, _ := pem.Decode([]byte(responseBody.Response.Certificates[3].Certificate))
	certs.Mgmt.Certificate = make([][]byte, 1)
	certs.Mgmt.Certificate[0] = mgmtCertDER.Bytes
	alertsCertDER, _ := pem.Decode([]byte(responseBody.Response.Certificates[4].Certificate))
	certs.Alerts.Certificate = make([][]byte, 1)
	certs.Alerts.Certificate[0] = alertsCertDER.Bytes

	return certs
}

func sendRequest(reqBody []byte) (respBody []byte) {
	client := &http.Client{}
	r := bytes.NewReader(reqBody)

	req, err := http.NewRequest("POST", "https://identity.apple.com/pushcert/caservice/new", r)
	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Set("Content-Type", "text/x-xml-plist")
	req.Header.Set("User-Agent", "Servermgrd%20Plugin/6.0 CFNetwork/811.11 Darwin/16.7.0 (x86_64)")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-us")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}

	defer resp.Body.Close()
	respBody, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	if resp.StatusCode != 200 {
		log.Fatalf("Apple didn't return 200: %s", respBody)
	}
	return
}
