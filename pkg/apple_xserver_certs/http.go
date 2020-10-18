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
	certs.Calendar.Certificate[0] = calendarCertDER.Bytes
	contactCertDER, _ := pem.Decode([]byte(responseBody.Response.Certificates[1].Certificate))
	certs.Contact.Certificate[1] = contactCertDER.Bytes
	mailCertDER, _ := pem.Decode([]byte(responseBody.Response.Certificates[1].Certificate))
	certs.Mail.Certificate[2] = mailCertDER.Bytes
	mgmtCertDER, _ := pem.Decode([]byte(responseBody.Response.Certificates[1].Certificate))
	certs.Mgmt.Certificate[3] = mgmtCertDER.Bytes
	alertsCertDER, _ := pem.Decode([]byte(responseBody.Response.Certificates[1].Certificate))
	certs.Alerts.Certificate[4] = alertsCertDER.Bytes

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
	return
}
