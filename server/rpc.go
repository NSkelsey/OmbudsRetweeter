package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/btcsuite/btcd/btcjson/v2/btcjson"
	_ "github.com/soapboxsys/ombudslib/rpcexten"
)

// newHTTPClient returns a new HTTP client that is configured according to the
// proxy and TLS settings in the associated connection configuration.
func newHTTPClient(cfg *config) (*http.Client, error) {
	// Configure proxy if needed.
	var dial func(network, addr string) (net.Conn, error)

	// Configure TLS if needed.
	var tlsConfig *tls.Config
	if !cfg.NoTLS && cfg.RPCCert != "" {
		pem, err := ioutil.ReadFile(cfg.RPCCert)
		if err != nil {
			return nil, err
		}

		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(pem)
		tlsConfig = &tls.Config{
			RootCAs: pool,
		}
	}

	// Create and return the new HTTP client potentially configured with a
	// proxy and TLS.
	client := http.Client{
		Transport: &http.Transport{
			Dial:            dial,
			TLSClientConfig: tlsConfig,
		},
	}
	return &client, nil
}

// sendPostRequest sends the marshalled JSON-RPC command using HTTP-POST mode
// to the server described in the passed config struct.  It also attempts to
// unmarshal the response as a JSON-RPC response and returns either the result
// field or the error field depending on whether or not there is an error.
func sendPostRequest(marshalledJSON []byte, cfg *config) ([]byte, error) {
	// Generate a request to the configured RPC server.
	protocol := "http"
	if !cfg.NoTLS {
		protocol = "https"
	}
	url := protocol + "://" + cfg.RPCServer
	bodyReader := bytes.NewReader(marshalledJSON)
	httpRequest, err := http.NewRequest("POST", url, bodyReader)
	if err != nil {
		return nil, err
	}
	httpRequest.Close = true
	httpRequest.Header.Set("Content-Type", "application/json")

	// Configure basic access authorization.
	httpRequest.SetBasicAuth(cfg.RPCUser, cfg.RPCPassword)

	// Create the new HTTP client that is configured according to the user-
	// specified options and submit the request.
	httpClient, err := newHTTPClient(cfg)
	if err != nil {
		return nil, err
	}
	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	// Read the raw bytes and close the response.
	respBytes, err := ioutil.ReadAll(httpResponse.Body)
	httpResponse.Body.Close()
	if err != nil {
		err = fmt.Errorf("error reading json reply: %v", err)
		return nil, err
	}

	// Handle unsuccessful HTTP responses
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		// Generate a standard error to return if the server body is
		// empty.  This should not happen very often, but it's better
		// than showing nothing in case the target server has a poor
		// implementation.
		if len(respBytes) == 0 {
			return nil, fmt.Errorf("%d %s", httpResponse.StatusCode,
				http.StatusText(httpResponse.StatusCode))
		}
		return nil, fmt.Errorf("%s", respBytes)
	}

	// Unmarshal the response.
	var resp btcjson.Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Result, nil
}

func (s *server) rpcsend(cmd interface{}) (string, error) {
	// Marshal the command into a JSON-RPC byte slice in preparation for
	// sending it to the RPC server.

	s.cnt += 1
	marshalledJSON, err := btcjson.MarshalCmd(s.cnt, cmd)
	if err != nil {
		return "", err
	}

	// Send the JSON-RPC request to the server using the user-specified
	// connection configuration.
	result, err := sendPostRequest(marshalledJSON, s.cfg)
	if err != nil {
		return "", err
	}

	// Choose how to display the result based on its type.
	strResult := string(result)
	if strings.HasPrefix(strResult, "{") || strings.HasPrefix(strResult, "[") {
		// There was an error
		var dst bytes.Buffer
		if err := json.Indent(&dst, result, "", "  "); err != nil {
			return "", fmt.Errorf("Failed to format result: %v",
				err)
		}
		return "", fmt.Errorf("Did not expect a JSON response: %s", dst.String())

	} else if strings.HasPrefix(strResult, `"`) {
		var s string = ""
		err = json.Unmarshal(result, &s)
		if err != nil {
			return "", err
		}
		return s, nil

	} else if strResult != "null" {
		// Send seems like it suceeded
		log.Println(strResult)
		return strResult, nil
	}

	// Empty json response
	return "", nil
}
