// repeater.go

package app

import (
	"log"
	"testing"
	"time"
)

func TestSendHTTP2RawRequest(t *testing.T) {
	// Test cases
	testCases := []RawRequest{
		{
			TLS:      true,
			Hostname: "accounts.google.com",
			Port:     "443",
			Timeout:  time.Duration(10) * time.Second,
			Request: `GET /v3/signin/identifier?continue=https%3A%2F%2Fwww.youtube.com%2Fsignin%3Faction_handle_signin%3Dtrue%26app%3Ddesktop%26hl%3Den%26next%3D%252Fsignin_passive%26feature%3Dpassive&hl=en&ifkv=AeDOFXikiBl_sUHI_qVRGmA6XaTbbzLMfaLLknbthR57EhiaAbiU-ldTlhj_JS9FWIvLD_-4Ukg7&passive=true&service=youtube&uilel=3&flowName=GlifWebSignIn&flowEntry=ServiceLogin&dsh=S-1794285892%3A1690707286232914 HTTP/2.0
Host: accounts.google.com
User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/115.0
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8
Accept-Encoding: gzip, deflate, br
Accept-Language: en-US,en;q=0.5
Connection: keep-alive
Cookie: __Host-GAPS=1:V1a5N9uqr4ITJ7uavISIcsjFzTCT-w:Eqnu-xZHShYWvR33
Referer: https://www.youtube.com/
Sec-Fetch-Dest: iframe
Sec-Fetch-Mode: navigate
Sec-Fetch-Site: cross-site
Upgrade-Insecure-Requests: 1

`,
		},
	}

	// Test each case
	for _, tc := range testCases {
		respString, time, err := SendHTTP2RawRequest(tc)
		if err != nil {
			log.Println(err)
		}
		log.Println(respString)
		log.Println(time)
	}
}
