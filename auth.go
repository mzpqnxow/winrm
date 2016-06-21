package winrm

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/Azure/azure-sdk-for-go/core/http"
	"github.com/Azure/azure-sdk-for-go/core/tls"

	"github.com/masterzen/winrm/soap"
)

type clientAuthRequest struct {
	transport http.RoundTripper
}

func (c *clientAuthRequest) Transport(endpoint *Endpoint) error {
	cert, err := tls.X509KeyPair(endpoint.Cert, endpoint.Key)
	if err != nil {
		return err
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: endpoint.Insecure,
			Certificates:       []tls.Certificate{cert},
		},
		ResponseHeaderTimeout: endpoint.Timeout,
	}

	if endpoint.CACert != nil && len(endpoint.CACert) > 0 {
		certPool, err := readCACerts(endpoint.CACert)
		if err != nil {
			return err
		}

		transport.TLSClientConfig.RootCAs = certPool
	}

	c.transport = transport

	return nil
}

// parse func reads the response body and return it as a string
func parse(response *http.Response) (string, error) {

	// if we recived the content we expected
	if strings.Contains(response.Header.Get("Content-Type"), "application/soap+xml") {
		body, err := ioutil.ReadAll(response.Body)
		defer func() {
			// defer can modify the returned value before
			// it is actually passed to the calling statement
			if errClose := response.Body.Close(); errClose != nil && err == nil {
				err = errClose
			}
		}()
		if err != nil {
			return "", fmt.Errorf("error while reading request body %s", err)
		}

		return string(body), nil
	}

	return "", fmt.Errorf("invalid content type")
}

func (c clientAuthRequest) Post(client *Client, request *soap.SoapMessage) (string, error) {
	httpClient := &http.Client{Transport: c.transport}

	req, err := http.NewRequest("POST", client.url, strings.NewReader(request.String()))
	if err != nil {
		return "", fmt.Errorf("impossible to create http request %s", err)
	}

	req.Header.Set("Content-Type", soapXML+";charset=UTF-8")
	req.Header.Set("Authorization", "http://schemas.dmtf.org/wbem/wsman/1/wsman/secprofile/https/mutual")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("unknown error %s", err)
	}

	body, err := parse(resp)
	if err != nil {
		return "", fmt.Errorf("http response error: %d - %s", resp.StatusCode, err.Error())
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("http error: %d - %s", resp.StatusCode, body)
	}

	return body, err
}
