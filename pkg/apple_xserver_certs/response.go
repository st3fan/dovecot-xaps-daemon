package apple_xserver_certs

import (
	"bytes"
	"github.com/freswa/go-plist"
)

type (
	aplRespBody struct {
		Response aplResponse
		Header   aplHeader
	}

	aplResponse struct {
		Status       aplStatus
		Certificates []aplCert
	}

	aplStatus struct {
		ErrorDescription string
		ErrorMessage     string
		ErrorCode        int
	}

	aplCert struct {
		Certificate   string
		CertRequestNo int
		Description   string
		ServiceType   string
	}

	aplHeader struct {
		ClientIPAddress             string
		LanguagePreference          string
		TransactionId               string
		ClientOSVersion             string
		ClientOSName                string
		ClientApplicationName       string
		ClientApplicationCredential string
	}
)

func parseResponse(respBody []byte) (aplRespBody, error) {
	buf := bytes.NewReader(respBody)
	decoder := plist.NewDecoder(buf)

	var parsedBody aplRespBody
	err := decoder.Decode(&parsedBody)
	if err != nil {
		return aplRespBody{}, err
	}
	return parsedBody, nil
}
